package launcher

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// Archive formats.
const (
	formatZip   = "zip"
	formatTarGz = "tar.gz"
)

// extract unpacks the archive at src into dest, dropping the first strip components of
// every entry's path. An entry that would land outside dest is an error, not a skip.
func extract(src, format, dest string, strip int) error {
	switch format {
	case formatZip:
		return extractZip(src, dest, strip)
	case formatTarGz:
		return extractTarGz(src, dest, strip)
	}
	return fmt.Errorf("unknown archive format %q", format)
}

// entryPath returns where an archive entry named name goes under dest, or "" for an entry
// that strip leaves nothing of.
func entryPath(dest, name string, strip int) (string, error) {
	if strings.Contains(name, `\`) || path.IsAbs(name) || slices.Contains(strings.Split(name, "/"), "..") {
		return "", fmt.Errorf("archive entry %q escapes the destination", name)
	}
	parts := strings.Split(strings.Trim(path.Clean("/"+name), "/"), "/")
	if len(parts) <= strip {
		return "", nil
	}
	rel := filepath.FromSlash(strings.Join(parts[strip:], "/"))
	if !filepath.IsLocal(rel) {
		return "", fmt.Errorf("archive entry %q escapes the destination", name)
	}
	return filepath.Join(dest, rel), nil
}

func extractZip(src, dest string, strip int) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		target, err := entryPath(dest, f.Name, strip)
		if err != nil || target == "" {
			if err != nil {
				return err
			}
			continue
		}
		mode := f.Mode()
		switch {
		case mode.IsDir():
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case mode&fs.ModeSymlink != 0:
			return fmt.Errorf("archive entry %q is a symbolic link, which a zip here never holds", f.Name)
		default:
			rc, err := f.Open()
			if err != nil {
				return err
			}
			err = writeFile(target, rc, mode.Perm())
			rc.Close()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func extractTarGz(src, dest string, strip int) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := entryPath(dest, h.Name, strip)
		if err != nil {
			return err
		}
		if target == "" {
			continue
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := writeFile(target, tr, fs.FileMode(h.Mode).Perm()); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := symlink(dest, target, h.Name, h.Linkname); err != nil {
				return err
			}
		}
	}
}

// symlink creates target pointing at linkname, which must be relative and resolve inside dest.
func symlink(dest, target, name, linkname string) error {
	resolved := filepath.Join(filepath.Dir(target), filepath.FromSlash(linkname))
	rel, err := filepath.Rel(dest, resolved)
	rooted := path.IsAbs(linkname) || filepath.IsAbs(linkname) || filepath.VolumeName(linkname) != "" || strings.HasPrefix(linkname, `\`)
	if err != nil || rooted || !filepath.IsLocal(rel) {
		return fmt.Errorf("archive entry %q links outside the destination, to %q", name, linkname)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.Symlink(linkname, target)
}

func writeFile(target string, r io.Reader, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm|0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
