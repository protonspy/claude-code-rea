package launcher

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/zalando/go-keyring"
	"golang.org/x/term"
)

// DefaultSystem returns the System of a real run: the user's cache and configuration
// directories, the process's environment and terminal, the network, the operating system's
// credential store, and the browser.
//
// There is no fallback to the temporary directory: on a shared machine another user could
// put a toolchain there for the launcher to take as installed.
func DefaultSystem(version string) (System, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return System{}, fmt.Errorf("no user cache directory for the toolchain: %w", err)
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return System{}, fmt.Errorf("no user configuration directory for the settings: %w", err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = time.Minute
	sys := System{
		Root:             filepath.Join(cache, "rea-launcher"),
		ConfigDir:        filepath.Join(config, "rea-launcher"),
		Platform:         Platform{runtime.GOOS, runtime.GOARCH},
		Environ:          os.Environ(),
		Stdin:            os.Stdin,
		Stdout:           os.Stdout,
		Stderr:           os.Stderr,
		HTTP:             &http.Client{Transport: transport},
		Store:            keyringStore{service: "rea-launcher"},
		OpenURL:          openBrowser,
		Run:              runCommand,
		IgnoreInterrupts: ignoreInterrupts,
		Pins:             DefaultPins(),
		AuthURL:          nanoGPTAuthURL,
		TokenURL:         nanoGPTTokenURL,
		SignInTimeout:    signInTimeout,
		SignInPort:       signInPort,
		Version:          version,
	}
	if term.IsTerminal(int(os.Stdin.Fd())) {
		sys.Prompt = readSecret
	}
	return sys, nil
}

// ignoreInterrupts keeps an interrupt from stopping the launcher while OpenCode runs. It
// catches the signal rather than ignoring it: a handler is reset across exec, so OpenCode
// starts with the default disposition, where an ignored signal would be inherited.
func ignoreInterrupts() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		for range ch {
		}
	}()
}

// readSecret reads a line from the terminal without echoing it.
func readSecret(label string) (string, error) {
	fmt.Fprint(os.Stderr, label)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	return string(b), err
}

// runCommand runs c and returns its exit code; the error is for a command that could not
// be started.
func runCommand(ctx context.Context, c Command) (int, error) {
	cmd := exec.CommandContext(ctx, c.Path, c.Args...)
	cmd.Env = c.Env
	cmd.Dir = c.Dir
	cmd.Stdin = c.Stdin
	cmd.Stdout = c.Stdout
	cmd.Stderr = c.Stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if ws, ok := exit.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return 128 + int(ws.Signal()), nil // the shell's convention for a death by signal
		}
		return exit.ExitCode(), nil
	}
	if err != nil {
		return -1, err
	}
	return 0, nil
}

// openBrowser opens url in the user's browser without waiting for it.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

// keyringStore keeps keys in the operating system's credential store.
type keyringStore struct{ service string }

func (k keyringStore) Get(account string) (string, error) {
	v, err := keyring.Get(k.service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNoKey
	}
	return v, err
}

func (k keyringStore) Set(account, secret string) error {
	return keyring.Set(k.service, account, secret)
}

func (k keyringStore) Delete(account string) error {
	err := keyring.Delete(k.service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNoKey
	}
	return err
}
