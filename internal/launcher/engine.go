package launcher

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// The Ghidra releases REA 6.10.1 admits: the 12 line from 12.2 up, the first whose
// decompiler API its bridge compiles against (dist/ghidra/GhidraReleasePolicy.js,
// SUPPORTED_GHIDRA_VERSION and ADMITTED_GHIDRA_MINOR_FLOOR).
const (
	ghidraMajor      = 12
	ghidraMinorFloor = 2
)

var ghidraRelease = regexp.MustCompile(`^(\d+)\.(\d+)(?:\.(\d+))?([-+].*)?$`)

// ghidraInstall is what a Ghidra installation declares about itself.
type ghidraInstall struct {
	Version string
	JavaMin int // the oldest JDK major it runs on; 0 when it declares none
}

// readSmall reads at most 1 MiB of the regular file at path: an installation's metadata
// files are a few hundred bytes, and a pipe or a device would never end.
func readSmall(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil {
		return nil, err
	} else if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	return io.ReadAll(io.LimitReader(f, 1<<20))
}

// readGhidra reads the release and the JDK floor dir's application.properties declares.
func readGhidra(dir string) (ghidraInstall, error) {
	data, err := readSmall(filepath.Join(dir, "Ghidra", "application.properties"))
	if err != nil {
		return ghidraInstall{}, fmt.Errorf("%s is not a Ghidra installation: %w", dir, err)
	}
	props := map[string]string{}
	s := bufio.NewScanner(bytes.NewReader(data))
	for s.Scan() {
		if k, v, ok := strings.Cut(s.Text(), "="); ok && !strings.HasPrefix(k, "#") {
			props[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	g := ghidraInstall{Version: props["application.version"]}
	if g.Version == "" {
		return g, fmt.Errorf("%s declares no Ghidra version", dir)
	}
	if v := props["application.java.min"]; v != "" {
		if g.JavaMin, err = strconv.Atoi(v); err != nil {
			return g, fmt.Errorf("%s declares JDK %q as its minimum, which is not a number", dir, v)
		}
	}
	return g, nil
}

// admitted reports whether REA admits Ghidra release version.
func admitted(version string) bool {
	m := ghidraRelease.FindStringSubmatch(version)
	if m == nil {
		return false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	return major == ghidraMajor && minor >= ghidraMinorFloor
}

var javaVersion = regexp.MustCompile(`(?m)^JAVA_VERSION="(\d+)(?:\.(\d+))?`)

// readJDK returns the major version of the JDK in dir, from its release file.
func readJDK(dir string) (int, error) {
	data, err := readSmall(filepath.Join(dir, "release"))
	if err != nil {
		return 0, fmt.Errorf("%s is not a JDK: %w", dir, err)
	}
	m := javaVersion.FindSubmatch(data)
	if m == nil {
		return 0, fmt.Errorf("%s declares no JAVA_VERSION in its release file", dir)
	}
	major, _ := strconv.Atoi(string(m[1]))
	if major == 1 && len(m[2]) > 0 {
		major, _ = strconv.Atoi(string(m[2]))
	}
	return major, nil
}

// lookup returns env's value of name, compared as the platform compares variable names.
func (s System) lookup(env map[string]string, name string) string {
	for k, v := range env {
		if s.sameVar(k, name) {
			return v
		}
	}
	return ""
}

// engineMismatch returns why the Ghidra and the JDK env gives REA cannot run together, or
// nil when they can or when env names no Ghidra. A JDK env does not name is left to REA's
// own search, and so is the JDK of a Ghidra that declares no floor.
func engineMismatch(sys System, env map[string]string) error {
	dir := sys.lookup(env, "GHIDRA_INSTALL_DIR")
	if dir == "" {
		return nil
	}
	g, err := readGhidra(dir)
	if err != nil {
		return err
	}
	if !admitted(g.Version) {
		return fmt.Errorf("Ghidra %q in %s is not one REA %s runs: it needs Ghidra %d.%d or a later %d.x release", g.Version, dir, REAVersion, ghidraMajor, ghidraMinorFloor, ghidraMajor)
	}
	home := sys.lookup(env, "JAVA_HOME")
	if home == "" || g.JavaMin == 0 {
		return nil
	}
	major, err := readJDK(home)
	if err != nil {
		return err
	}
	if major < g.JavaMin {
		return fmt.Errorf("JDK %d in %s is older than the JDK %d Ghidra %q needs", major, home, g.JavaMin, g.Version)
	}
	return nil
}
