package launcher

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type entry struct {
	name, body, link string
	dir              bool
	mode             int64
}

func zipBytes(t *testing.T, entries ...entry) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, e := range entries {
		f, err := w.Create(e.name)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte(e.body))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func tgzBytes(t *testing.T, entries ...entry) []byte {
	t.Helper()
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	w := tar.NewWriter(gz)
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Mode: 0o644, Size: int64(len(e.body)), Typeflag: tar.TypeReg}
		if e.mode != 0 {
			h.Mode = e.mode
		}
		switch {
		case e.dir:
			h.Typeflag, h.Size, h.Mode = tar.TypeDir, 0, 0o755
		case e.link != "":
			h.Typeflag, h.Size, h.Linkname = tar.TypeSymlink, 0, e.link
		}
		if err := w.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(e.body))
	}
	w.Close()
	gz.Close()
	return b.Bytes()
}

func writeTemp(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestZipIsUnpackedWithItsTopDirectoryStripped(t *testing.T) {
	src := writeTemp(t, "a.zip", zipBytes(t,
		entry{name: "node-v24/"},
		entry{name: "node-v24/node.exe", body: "exe"},
		entry{name: "node-v24/node_modules/npm/bin/npm-cli.js", body: "npm"},
	))
	dest := t.TempDir()
	if err := extract(src, formatZip, dest, 1); err != nil {
		t.Fatal(err)
	}
	if readFile(t, filepath.Join(dest, "node.exe")) != "exe" ||
		readFile(t, filepath.Join(dest, "node_modules", "npm", "bin", "npm-cli.js")) != "npm" {
		t.Error("unexpected content")
	}
}

func TestTarGzIsUnpackedWithModesAndInnerLinks(t *testing.T) {
	entries := []entry{
		{name: "package/", dir: true},
		{name: "package/bin/opencode", body: "bin", mode: 0o755},
	}
	if runtime.GOOS != "windows" { // a symbolic link needs a privilege there, and no Windows archive holds one
		entries = append(entries, entry{name: "package/bin/oc", link: "opencode"})
	}
	src := writeTemp(t, "a.tgz", tgzBytes(t, entries...))
	dest := t.TempDir()
	if err := extract(src, formatTarGz, dest, 1); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dest, "bin", "opencode")
	if readFile(t, bin) != "bin" {
		t.Error("unexpected content")
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(bin); info.Mode()&0o100 == 0 {
			t.Errorf("mode %v lost the executable bit", info.Mode())
		}
		if readFile(t, filepath.Join(dest, "bin", "oc")) != "bin" {
			t.Error("the inner link does not resolve")
		}
	}
}

func TestEntriesThatEscapeAreRefused(t *testing.T) {
	cases := map[string][]byte{
		"zip parent":    zipBytes(t, entry{name: "top/../../evil", body: "x"}),
		"zip backslash": zipBytes(t, entry{name: `top\..\..\evil`, body: "x"}),
		"tgz parent":    tgzBytes(t, entry{name: "package/../../evil", body: "x"}),
		"tgz link":      tgzBytes(t, entry{name: "package/bin/evil", link: "../../../etc/passwd"}),
		"tgz abs link":  tgzBytes(t, entry{name: "package/bin/evil", link: "/etc/passwd"}),
	}
	for name, data := range cases {
		format := formatTarGz
		if strings.HasPrefix(name, "zip") {
			format = formatZip
		}
		parent := t.TempDir()
		dest := filepath.Join(parent, "dest")
		os.Mkdir(dest, 0o755)
		if err := extract(writeTemp(t, "a", data), format, dest, 1); err == nil || !strings.Contains(err.Error(), "outside") && !strings.Contains(err.Error(), "escapes") {
			t.Errorf("%s: err = %v, want a refusal", name, err)
		}
		if _, err := os.Lstat(filepath.Join(parent, "evil")); err == nil {
			t.Errorf("%s: a file was written outside the destination", name)
		}
	}
}

func TestUnknownFormatIsAnError(t *testing.T) {
	if err := extract("x", "rar", t.TempDir(), 0); err == nil {
		t.Error("want an error")
	}
}
