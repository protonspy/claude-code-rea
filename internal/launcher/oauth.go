package launcher

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

// OAuth is NanoGPT's OAuth PKCE sign-in, which mints a dedicated API key.
type OAuth struct {
	AuthURL  string // the sign-in page
	TokenURL string // the endpoint that exchanges the code for a key
	Timeout  time.Duration
	Client   *http.Client
	Open     func(url string) error // opens url in the browser
	Stderr   io.Writer
}

// NanoGPT's shortcut flow, which needs no registered client.
const (
	nanoGPTAuthURL  = "https://nano-gpt.com/auth"
	nanoGPTTokenURL = "https://nano-gpt.com/api/v1/auth/keys"
	signInTimeout   = 5 * time.Minute
)

type callback struct {
	code string
	err  error
}

// SignIn runs the sign-in and returns the key NanoGPT issues.
func (o OAuth) SignIn(ctx context.Context) (string, error) {
	verifier, err := randomToken(64)
	if err != nil {
		return "", err
	}
	state, err := randomToken(32)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(verifier))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("listen for the sign-in callback: %w", err)
	}
	callbackURL := fmt.Sprintf("http://127.0.0.1:%d/callback", ln.Addr().(*net.TCPAddr).Port)

	results := make(chan callback, 1)
	mux := http.NewServeMux()
	host := ln.Addr().String()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		// Any local process or web page can reach this port. A request that is not the
		// browser coming back from this sign-in is refused and the sign-in keeps waiting,
		// so it cannot end the sign-in either.
		if r.Method != http.MethodGet || r.Host != host || !sameState(r.URL.Query(), state) {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "rea-launcher: not a callback from this sign-in")
			return
		}
		res := readCallback(r.URL.Query())
		if res.err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "rea-launcher: sign-in failed: %v\n", res.err)
		} else {
			fmt.Fprintln(w, "rea-launcher: signed in to NanoGPT. You can close this tab.")
		}
		select {
		case results <- res:
		default:
		}
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go srv.Serve(ln)
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(shutdown)
	}()

	q := url.Values{
		"callback_url":          {callbackURL},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(sum[:])},
		"code_challenge_method": {"S256"},
		"scope":                 {"api.use models.read"},
		"state":                 {state},
		"client_name":           {"rea-launcher"},
	}
	signInURL := o.AuthURL + "?" + q.Encode()
	fmt.Fprintf(o.Stderr, "rea-launcher: sign in to NanoGPT in your browser:\n  %s\n", signInURL)
	if err := o.Open(signInURL); err != nil {
		fmt.Fprintf(o.Stderr, "rea-launcher: could not open a browser (%v); open the address above by hand\n", err)
	}

	timer := time.NewTimer(o.Timeout)
	defer timer.Stop()
	var res callback
	select {
	case res = <-results:
	case <-timer.C:
		return "", fmt.Errorf("no sign-in within %s", o.Timeout)
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if res.err != nil {
		return "", res.err
	}
	return o.exchange(ctx, res.code, verifier)
}

// sameState reports whether the callback carries the state the sign-in sent.
func sameState(q url.Values, state string) bool {
	return subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) == 1
}

// readCallback reads the outcome of a callback that carried this sign-in's state.
func readCallback(q url.Values) callback {
	if e := q.Get("error"); e != "" {
		return callback{err: fmt.Errorf("NanoGPT refused the sign-in: %s", e)}
	}
	if q.Get("code") == "" {
		return callback{err: errors.New("the sign-in callback carried no code")}
	}
	return callback{code: q.Get("code")}
}

// exchange trades the code and its verifier for the key.
func (o OAuth) exchange(ctx context.Context, code, verifier string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"code_verifier": verifier,
	})
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.TokenURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("NanoGPT code exchange: %w", err)
	}
	defer resp.Body.Close()
	var out struct {
		Key         string `json:"key"`
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil && resp.StatusCode == http.StatusOK {
		return "", fmt.Errorf("NanoGPT code exchange: unreadable answer: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("NanoGPT code exchange failed: HTTP %d %s %s", resp.StatusCode, out.Error, out.Description)
	}
	if out.Key != "" {
		return out.Key, nil
	}
	if out.AccessToken != "" {
		return out.AccessToken, nil
	}
	return "", errors.New("NanoGPT code exchange returned no key")
}

// randomToken returns n random bytes, base64url-encoded without padding.
func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
