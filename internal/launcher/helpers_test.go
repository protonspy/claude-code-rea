package launcher

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"
)

// memStore is a credential store in memory; err, when set, is what every call returns.
type memStore struct {
	keys map[string]string
	err  error
	sets int
}

func newMemStore() *memStore { return &memStore{keys: map[string]string{}} }

func (m *memStore) Get(account string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	k, ok := m.keys[account]
	if !ok {
		return "", ErrNoKey
	}
	return k, nil
}

func (m *memStore) Set(account, secret string) error {
	if m.err != nil {
		return m.err
	}
	m.sets++
	m.keys[account] = secret
	return nil
}

func (m *memStore) Delete(account string) error {
	if m.err != nil {
		return m.err
	}
	if _, ok := m.keys[account]; !ok {
		return ErrNoKey
	}
	delete(m.keys, account)
	return nil
}

// testSystem is a System with no effect outside the test: a temporary managed directory,
// buffers for output, a store in memory, no terminal, no browser and a runner that fails.
type testIO struct {
	stdout, stderr *bytes.Buffer
	store          *memStore
}

func testSystem(t *testing.T, environ ...string) (System, testIO) {
	t.Helper()
	io := testIO{stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}, store: newMemStore()}
	sys := System{
		Root:          t.TempDir(),
		Platform:      Platform{runtime.GOOS, runtime.GOARCH},
		Environ:       environ,
		Stdin:         strings.NewReader(""),
		Stdout:        io.stdout,
		Stderr:        io.stderr,
		HTTP:          http.DefaultClient,
		Store:         io.store,
		OpenURL:       func(string) error { return errors.New("no browser in tests") },
		AuthURL:       "https://nano-gpt.example/auth",
		TokenURL:      "http://127.0.0.1:1",
		SignInTimeout: 5 * time.Second,
		Version:       "test",
		Run: func(_ context.Context, c Command) (int, error) {
			t.Fatalf("unexpected command %s %v", c.Path, c.Args)
			return 0, nil
		},
		IgnoreInterrupts: func() {},
	}
	return sys, io
}
