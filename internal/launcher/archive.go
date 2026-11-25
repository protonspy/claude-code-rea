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
// every entry's path. An entry that would land outside dest is an error, not a skip. Every
// write goes through an os.Root on dest, so not even a chain of links the archive itself
// created can lead a later entry outside it.
func extract(src, format, dest string, strip int) error {
	root, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer root.Close()
	switch format {
	case formatZip:
		return extractZip(src, root, strip)
	case formatTarGz:
		return extractTarGz(src, root, strip)
	}
	return fmt.Errorf("unknown archive format %q", format)
}

// entryPath returns where an archive entry named name goes, relative to the destination,
// or "" for an entry that strip leaves nothing of.
func entryPath(name string, strip int) (string, error) {
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
	return rel, nil
}

func extractZip(src string, root *os.Root, strip int) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		rel, err := entryPath(f.Name, strip)
		if err != nil {
			return err
		}
		if rel == "" {
			continue
		}
		mode := f.Mode()
		switch {
		case mode.IsDir():
			if err := root.MkdirAll(rel, 0o755); err != nil {
				return err
			}
		case mode&fs.ModeSymlink != 0:
			return fmt.Errorf("archive entry %q is a symbolic link, which a zip here never holds", f.Name)
		default:
			rc, err := f.Open()
			if err != nil {
				return err
			}
			err = writeRooted(root, rel, rc, mode.Perm())
			rc.Close()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func extractTarGz(src string, root *os.Root, strip int) error {
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
		rel, err := entryPath(h.Name, strip)
		if err != nil {
			return err
		}
		if rel == "" {
			continue
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(rel, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := writeRooted(root, rel, tr, fs.FileMode(h.Mode).Perm()); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := symlink(root, rel, h.Name, h.Linkname); err != nil {
				return err
			}
		}
	}
}

// symlink creates rel pointing at linkname, which must be relative and, read as written,
// resolve inside the root.
func symlink(root *os.Root, rel, name, linkname string) error {
	resolved := filepath.Join(filepath.Dir(rel), filepath.FromSlash(linkname))
	rooted := path.IsAbs(linkname) || filepath.IsAbs(linkname) || filepath.VolumeName(linkname) != "" || strings.HasPrefix(linkname, `\`)
	if rooted || !filepath.IsLocal(resolved) {
		return fmt.Errorf("archive entry %q links outside the destination, to %q", name, linkname)
	}
	if err := root.MkdirAll(filepath.Dir(rel), 0o755); err != nil {
		return err
	}
	return root.Symlink(linkname, rel)
}

// writeRooted writes r to rel inside root.
func writeRooted(root *os.Root, rel string, r io.Reader, perm fs.FileMode) error {
	if err := root.MkdirAll(filepath.Dir(rel), 0o755); err != nil {
		return err
	}
	out, err := root.OpenFile(rel, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm|0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		return err
	}
	return out.Close()
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
