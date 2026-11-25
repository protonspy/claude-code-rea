package launcher

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Artifact is a pinned download.
type Artifact struct {
	Name      string // what it is, for messages
	File      string // the file name it is saved under
	URL       string
	Integrity string // an SRI hash: "sha256-<base64>" or "sha512-<base64>"
	Format    string // formatZip or formatTarGz; empty for an archive npm installs
	Strip     int    // leading path components dropped when unpacking
}

// fetch downloads a into dir and returns its path once its content matches a.Integrity.
// A download that does not match is deleted.
func fetch(ctx context.Context, client *http.Client, a Artifact, dir string) (string, error) {
	algo, _, ok := strings.Cut(a.Integrity, "-")
	var h hash.Hash
	switch {
	case ok && algo == "sha256":
		h = sha256.New()
	case ok && algo == "sha512":
		h = sha512.New()
	default:
		return "", fmt.Errorf("%s: unsupported pin %q", a.Name, a.Integrity)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	idle := time.AfterFunc(downloadIdleTimeout, func() {
		cancel(fmt.Errorf("no data for %s", downloadIdleTimeout))
	})
	defer idle.Stop()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", a.Name, stalled(ctx, err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s from %s: HTTP %d", a.Name, a.URL, resp.StatusCode)
	}
	part, err := os.CreateTemp(dir, a.File+".*.part")
	if err != nil {
		return "", err
	}
	_, err = io.Copy(io.MultiWriter(part, h), idleReader{resp.Body, idle})
	if cerr := part.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(part.Name())
		return "", fmt.Errorf("download %s: %w", a.Name, stalled(ctx, err))
	}
	if got := algo + "-" + base64.StdEncoding.EncodeToString(h.Sum(nil)); got != a.Integrity {
		os.Remove(part.Name())
		return "", fmt.Errorf("%s from %s does not match its pinned hash and was deleted: expected %s, got %s", a.Name, a.URL, a.Integrity, got)
	}
	final := filepath.Join(dir, a.File)
	if err := renameRetrying(part.Name(), final); err != nil {
		os.Remove(part.Name())
		return "", err
	}
	return final, nil
}

// downloadIdleTimeout is how long a download may go without receiving data.
var downloadIdleTimeout = 2 * time.Minute

// idleReader restarts the idle timer on every read.
type idleReader struct {
	r     io.Reader
	timer *time.Timer
}

func (i idleReader) Read(p []byte) (int, error) {
	n, err := i.r.Read(p)
	i.timer.Reset(downloadIdleTimeout)
	return n, err
}

// stalled returns why ctx was cancelled when that explains err, and err otherwise.
func stalled(ctx context.Context, err error) error {
	if cause := context.Cause(ctx); cause != nil && !errors.Is(cause, context.Canceled) {
		return cause
	}
	return err
}

// publish installs dest by filling a temporary directory beside it and renaming that into
// place, so dest exists only once it is complete. An existing dest is an installed
// component and is left alone.
func publish(dest string, fill func(tmp string) error) error {
	if _, err := os.Stat(dest); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(dest), ".tmp-"+filepath.Base(dest)+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := fill(tmp); err != nil {
		return err
	}
	if err := renameRetrying(tmp, dest); err != nil {
		if _, statErr := os.Stat(dest); statErr == nil {
			return nil // another launcher installed it first
		}
		return err
	}
	return nil
}

// renameRetrying renames, retrying for a few seconds while the file is held open — on
// Windows a virus scanner opens a freshly written executable, and the rename fails until
// it lets go.
func renameRetrying(from, to string) error {
	var err error
	for wait := 100 * time.Millisecond; wait < 5*time.Second; wait *= 2 {
		if err = os.Rename(from, to); err == nil || errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if _, statErr := os.Stat(to); statErr == nil {
			return err
		}
		time.Sleep(wait)
	}
	return err
}

// ensureToolchain installs whatever of Node.js, OpenCode and REA is missing.
func ensureToolchain(ctx context.Context, sys System) error {
	l := sys.layout()
	node, opencode, err := sys.Pins.forPlatform(sys.Platform)
	if err != nil {
		return err
	}
	for _, c := range []struct {
		dest string
		a    Artifact
	}{{l.NodeDir(), node}, {l.OpenCodeDir(), opencode}} {
		if err := publish(c.dest, func(tmp string) error { return unpack(ctx, sys, c.a, tmp) }); err != nil {
			return fmt.Errorf("install %s: %w", c.a.Name, err)
		}
	}
	if err := publish(l.READir(), func(tmp string) error { return installREA(ctx, sys, tmp) }); err != nil {
		return fmt.Errorf("install %s: %w", sys.Pins.REA.Name, err)
	}
	return nil
}

// unpack downloads a, verifies it, and extracts it into dir.
func unpack(ctx context.Context, sys System, a Artifact, dir string) error {
	fmt.Fprintln(sys.Stderr, "rea-launcher: installing", a.Name)
	archive, err := fetch(ctx, sys.HTTP, a, sys.layout().Downloads())
	if err != nil {
		return err
	}
	defer os.Remove(archive)
	return extract(archive, a.Format, dir, a.Strip)
}

// installREA installs the verified REA tarball into dir with the managed npm, whose
// dependencies come from the tarball's npm-shrinkwrap.json, and copies REA's skills into
// the OpenCode configuration directory beside it.
func installREA(ctx context.Context, sys System, dir string) error {
	l := sys.layout()
	a := sys.Pins.REA
	fmt.Fprintln(sys.Stderr, "rea-launcher: installing", a.Name)
	archive, err := fetch(ctx, sys.HTTP, a, l.Downloads())
	if err != nil {
		return err
	}
	defer os.Remove(archive)
	pathName, pathValue := sys.pathWith(l.NodeBin())
	code, err := sys.Run(ctx, Command{
		Path:   l.Node(),
		Args:   []string{l.NPM(), "install", "--prefix", dir, "--omit=dev", "--no-audit", "--no-fund", "--loglevel=error", "--registry=https://registry.npmjs.org/", archive},
		Env:    sys.environWith(map[string]string{pathName: pathValue}, keyVars()...),
		Stdout: sys.Stderr,
		Stderr: sys.Stderr,
	})
	if err != nil {
		return fmt.Errorf("run npm: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("npm install exited with status %d", code)
	}
	pkg := filepath.Join(dir, "node_modules", "rea-agents")
	if _, err := os.Stat(filepath.Join(pkg, "scripts", "rea.mjs")); err != nil {
		return fmt.Errorf("npm installed no REA entry script: %w", err)
	}
	_, err = installSkills(pkg, filepath.Join(dir, "opencode", "skills"))
	return err
}
