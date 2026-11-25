package launcher

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// agentsMD is the reverse-engineering instructions every launcher session is given.
//
//go:embed agents.md
var agentsMD []byte

// writeAgents installs the embedded instructions at path, and reports whether it wrote them:
// a file already holding them is left alone.
func writeAgents(path string) (bool, error) {
	if current, err := os.ReadFile(path); err == nil && bytes.Equal(current, agentsMD) {
		return false, nil
	}
	if err := writeAtomic(path, agentsMD, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// writeAtomic replaces path with data through a temporary file beside it, so a reader sees
// the old content or the new, never a part.
func writeAtomic(path string, data []byte, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	_, err = tmp.Write(data)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp.Name(), perm)
	}
	if err == nil {
		err = renameRetrying(tmp.Name(), path)
	}
	if err != nil {
		os.Remove(tmp.Name())
	}
	return err
}

// installSkills copies every skill the REA package at pkg ships into skillsDir, and
// returns how many it copied.
func installSkills(pkg, skillsDir string) (int, error) {
	src := filepath.Join(pkg, "skills")
	entries, err := os.ReadDir(src)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(src, e.Name(), "SKILL.md")); err != nil {
			continue
		}
		if err := copyDir(filepath.Join(src, e.Name()), filepath.Join(skillsDir, e.Name())); err != nil {
			return n, err
		}
		n++
	}
	if n == 0 {
		return 0, errors.New("the REA package ships no skills")
	}
	return n, nil
}

// countSkills returns how many skills are installed in skillsDir.
func countSkills(skillsDir string) int {
	matches, _ := filepath.Glob(filepath.Join(skillsDir, "*", "SKILL.md"))
	return len(matches)
}

// copyDir copies the regular files and directories under src to dst.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case d.Type().IsRegular():
			in, err := os.Open(p)
			if err != nil {
				return err
			}
			defer in.Close()
			return writeFile(target, in, 0o644)
		}
		return nil
	})
}

// engineSettings is the file holding the engine paths setup was given.
func engineSettings(sys System) string { return filepath.Join(sys.ConfigDir, "engine.json") }

func loadEngineSettings(sys System) (map[string]string, error) {
	settings := map[string]string{}
	data, err := os.ReadFile(engineSettings(sys))
	if errors.Is(err, fs.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("%s: %w", engineSettings(sys), err)
	}
	return settings, nil
}

// saveEngineSettings adds set to the saved engine settings.
func saveEngineSettings(sys System, set map[string]string) error {
	settings, err := loadEngineSettings(sys)
	if err != nil {
		return err
	}
	for k, v := range set {
		settings[k] = v
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(engineSettings(sys), append(data, '\n'), 0o600)
}

// reaEnvironment returns the environment REA's server runs with: the saved engine settings,
// overridden by JAVA_HOME and any GHIDRA_* or REA_* variable of the launcher's environment.
func reaEnvironment(sys System) (map[string]string, error) {
	env, err := loadEngineSettings(sys)
	if err != nil {
		return nil, err
	}
	for _, kv := range sys.Environ {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			continue
		}
		name := k
		if sys.Platform.OS == "windows" {
			name = strings.ToUpper(k)
		}
		if name == "JAVA_HOME" || strings.HasPrefix(name, "GHIDRA_") || strings.HasPrefix(name, "REA_") {
			for saved := range env {
				if sys.sameVar(saved, k) {
					delete(env, saved)
				}
			}
			env[k] = v
		}
	}
	return env, nil
}

// checkEngine runs REA's doctor for Ghidra and returns whether the engine is ready and, if
// it is not, one line per failing check saying what is missing and how REA says to fix it.
func checkEngine(ctx context.Context, sys System, env map[string]string) (bool, []string, error) {
	l := sys.layout()
	var out bytes.Buffer
	code, err := sys.Run(ctx, Command{
		Path:   l.Node(),
		Args:   []string{l.REAScript(), "doctor", "--provider", "ghidra", "--json"},
		Env:    sys.environWith(env),
		Stdout: &out,
		Stderr: io.Discard,
	})
	if err != nil {
		return false, nil, fmt.Errorf("run REA's doctor: %w", err)
	}
	var report struct {
		Healthy     bool `json:"healthy"`
		ScopeChecks []struct {
			Name           string `json:"name"`
			OK             bool   `json:"ok"`
			Classification string `json:"classification"`
			Detail         string `json:"detail"`
			Remediation    string `json:"remediation"`
		} `json:"scope_checks"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		return false, nil, fmt.Errorf("REA's doctor exited with status %d and no readable report: %w", code, err)
	}
	if report.Healthy {
		return true, nil, nil
	}
	var missing []string
	for _, c := range report.ScopeChecks {
		if c.OK {
			continue
		}
		line := c.Name + ": " + c.Classification
		if c.Detail != "" {
			line += " — " + c.Detail
		}
		if c.Remediation != "" {
			line += ". " + c.Remediation
		}
		missing = append(missing, line)
	}
	return false, missing, nil
}
