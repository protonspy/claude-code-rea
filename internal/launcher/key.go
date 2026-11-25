package launcher

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Store keeps provider keys between runs, one per provider name.
type Store interface {
	Get(account string) (string, error) // ErrNoKey when none is stored
	Set(account, secret string) error
	Delete(account string) error
}

// ErrNoKey is a Store's answer for an account that holds no key.
var ErrNoKey = errors.New("no key stored")

// resolveKey returns p's key and where it came from: the provider's variable, the store,
// or — when neither has one — NanoGPT's sign-in or a hidden prompt, saved to the store.
func resolveKey(ctx context.Context, p Provider, sys System) (key, source string, err error) {
	if v := sys.getenv(p.KeyVar); v != "" {
		return v, "from " + p.KeyVar, nil
	}
	storeUp := true
	switch stored, err := sys.Store.Get(p.Name); {
	case err == nil && stored != "":
		return stored, "from the credential store", nil
	case err != nil && !errors.Is(err, ErrNoKey):
		storeUp = false
		fmt.Fprintf(sys.Stderr, "rea-launcher: warning: the credential store cannot be reached (%v); the key is kept for this run only\n", err)
	}

	var how string
	if p.SignIn {
		key, err = sys.oauth().SignIn(ctx)
		how = "signed in"
	} else {
		key, err = promptKey(p, sys)
		how = "entered"
	}
	if err != nil {
		return "", "", err
	}
	if !storeUp {
		return key, how + ", for this run only", nil
	}
	if err := sys.Store.Set(p.Name, key); err != nil {
		fmt.Fprintf(sys.Stderr, "rea-launcher: warning: the key could not be saved in the credential store (%v); it is kept for this run only\n", err)
		return key, how + ", for this run only", nil
	}
	return key, how + " and saved in the credential store", nil
}

// promptKey reads p's key from the terminal without echoing it.
func promptKey(p Provider, sys System) (string, error) {
	if sys.Prompt == nil {
		return "", fmt.Errorf("no %s key: set %s, or run rea-launcher from a terminal to type it", p.Label, p.KeyVar)
	}
	key, err := sys.Prompt(fmt.Sprintf("%s key (%s): ", p.Label, p.KeyVar))
	if err != nil {
		return "", fmt.Errorf("read the %s key: %w", p.Label, err)
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("no %s key was entered", p.Label)
	}
	return key, nil
}

// forgetKey deletes p's key from the store, reporting whether there was one.
func forgetKey(p Provider, sys System) (bool, error) {
	switch err := sys.Store.Delete(p.Name); {
	case err == nil:
		return true, nil
	case errors.Is(err, ErrNoKey):
		return false, nil
	default:
		return false, fmt.Errorf("delete the %s key from the credential store: %w", p.Label, err)
	}
}
