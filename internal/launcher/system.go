package launcher

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

// System is every effect the launcher has on the world, so a test can replace each one.
type System struct {
	Root             string // the managed directory
	Platform         Platform
	Environ          []string // the launcher's environment, as os.Environ returns it
	Stdin            io.Reader
	Stdout           io.Writer
	Stderr           io.Writer
	HTTP             *http.Client
	Store            Store
	Prompt           func(label string) (string, error) // reads a line without echo; nil when stdin is not a terminal
	OpenURL          func(url string) error
	Run              Runner
	IgnoreInterrupts func()
	AuthURL          string // NanoGPT's sign-in page
	TokenURL         string // NanoGPT's code exchange
	SignInTimeout    time.Duration
	Version          string
}

// Platform is an operating system and an architecture, as GOOS and GOARCH name them.
type Platform struct{ OS, Arch string }

func (p Platform) String() string { return p.OS + "/" + p.Arch }

// Command is a process to run.
type Command struct {
	Path   string
	Args   []string
	Env    []string
	Dir    string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Runner runs a command to completion and returns its exit code. The error is for a
// command that could not be run at all.
type Runner func(ctx context.Context, c Command) (int, error)

// getenv returns the value of name in the launcher's environment, matched without regard
// to case on Windows, where variable names are case-insensitive.
func (s System) getenv(name string) string {
	for _, kv := range s.Environ {
		k, v, ok := strings.Cut(kv, "=")
		if ok && s.sameVar(k, name) {
			return v
		}
	}
	return ""
}

func (s System) sameVar(a, b string) bool {
	if s.Platform.OS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func (s System) oauth() OAuth {
	return OAuth{
		AuthURL:  s.AuthURL,
		TokenURL: s.TokenURL,
		Timeout:  s.SignInTimeout,
		Client:   s.HTTP,
		Open:     s.OpenURL,
		Stderr:   s.Stderr,
	}
}
