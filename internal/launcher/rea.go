package launcher

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

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
