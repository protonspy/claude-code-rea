package launcher

import (
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func sri512(data []byte) string {
	sum := sha512.Sum512(data)
	return "sha512-" + base64.StdEncoding.EncodeToString(sum[:])
}

// fileServer serves files by path and counts the requests.
type fileServer struct {
	files map[string][]byte
	hits  atomic.Int32
}

func (f *fileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.hits.Add(1)
	data, ok := f.files[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Write(data)
}

func serve(t *testing.T, files map[string][]byte) (*fileServer, string) {
	t.Helper()
	fs := &fileServer{files: files}
	srv := httptest.NewServer(fs)
	t.Cleanup(srv.Close)
	return fs, srv.URL
}

func TestFetchKeepsAnArchiveWhoseHashMatches(t *testing.T) {
	data := []byte("the archive")
	_, base := serve(t, map[string][]byte{"/a.tgz": data})
	dir := t.TempDir()
	got, err := fetch(t.Context(), http.DefaultClient, Artifact{Name: "a", File: "a.tgz", URL: base + "/a.tgz", Integrity: sri512(data)}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if readFile(t, got) != "the archive" {
		t.Error("unexpected content")
	}
}

func TestFetchDeletesAnArchiveWhoseHashDiffers(t *testing.T) {
	_, base := serve(t, map[string][]byte{"/a.tgz": []byte("tampered")})
	dir := t.TempDir()
	want := sri512([]byte("the archive"))
	_, err := fetch(t.Context(), http.DefaultClient, Artifact{Name: "a", File: "a.tgz", URL: base + "/a.tgz", Integrity: want}, dir)
	if err == nil {
		t.Fatal("want an error for a hash mismatch")
	}
	if !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), sri512([]byte("tampered"))) {
		t.Errorf("error %q does not name the expected and actual hashes", err)
	}
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Errorf("the download was kept: %v", left)
	}
}

func TestFetchRefusesAnUnknownHashAlgorithm(t *testing.T) {
	_, base := serve(t, map[string][]byte{"/a.tgz": []byte("x")})
	_, err := fetch(t.Context(), http.DefaultClient, Artifact{Name: "a", File: "a.tgz", URL: base + "/a.tgz", Integrity: "md5-AAAA"}, t.TempDir())
	if err == nil {
		t.Fatal("want an error for an md5 pin")
	}
}

func TestFetchReportsAMissingArchive(t *testing.T) {
	_, base := serve(t, nil)
	_, err := fetch(t.Context(), http.DefaultClient, Artifact{Name: "a", File: "a.tgz", URL: base + "/a.tgz", Integrity: sri512(nil)}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v, want HTTP 404", err)
	}
}

func TestPublishMakesTheDirectoryVisibleOnlyWhenComplete(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "node-24.22.3")
	err := publish(dest, func(tmp string) error {
		os.WriteFile(filepath.Join(tmp, "half"), []byte("x"), 0o644)
		return errors.New("interrupted")
	})
	if err == nil {
		t.Fatal("want the fill error")
	}
	if left, _ := os.ReadDir(root); len(left) != 0 {
		t.Errorf("a failed install left %v", left)
	}
	if err := publish(dest, func(tmp string) error {
		return os.WriteFile(filepath.Join(tmp, "node"), []byte("x"), 0o644)
	}); err != nil {
		t.Fatal(err)
	}
	if readFile(t, filepath.Join(dest, "node")) != "x" {
		t.Error("the published directory lacks its content")
	}
}

func TestPublishLeavesAnInstalledDirectoryAlone(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "rea-6.10.1")
	os.Mkdir(dest, 0o755)
	called := false
	if err := publish(dest, func(string) error { called = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("an installed component was installed again")
	}
}

func TestAStalledDownloadIsAbandoned(t *testing.T) {
	defer func(d time.Duration) { downloadIdleTimeout = d }(downloadIdleTimeout)
	downloadIdleTimeout = 100 * time.Millisecond
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("part of it"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()
	dir := t.TempDir()
	_, err := fetch(t.Context(), http.DefaultClient, Artifact{Name: "a", File: "a.tgz", URL: srv.URL, Integrity: sri512(nil)}, dir)
	if err == nil || !strings.Contains(err.Error(), "no data") {
		t.Fatalf("err = %v, want a stall", err)
	}
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Errorf("the partial download was kept: %v", left)
	}
}
