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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", a.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s from %s: HTTP %d", a.Name, a.URL, resp.StatusCode)
	}
	part, err := os.CreateTemp(dir, a.File+".*.part")
	if err != nil {
		return "", err
	}
	_, err = io.Copy(io.MultiWriter(part, h), resp.Body)
	if cerr := part.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(part.Name())
		return "", fmt.Errorf("download %s: %w", a.Name, err)
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
