package launcher

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeNanoGPT is NanoGPT's key endpoint: it hands out key for code when the verifier
// matches the challenge the browser was sent.
type fakeNanoGPT struct {
	challenge string
	code      string
	key       string
	status    int
	calls     atomic.Int32
}

func (f *fakeNanoGPT) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.calls.Add(1)
	var body struct {
		GrantType    string `json:"grant_type"`
		Code         string `json:"code"`
		CodeVerifier string `json:"code_verifier"`
	}
	if r.Method != http.MethodPost || json.NewDecoder(r.Body).Decode(&body) != nil {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}
	sum := sha256.Sum256([]byte(body.CodeVerifier))
	if f.status != 0 || body.GrantType != "authorization_code" || body.Code != f.code ||
		base64.RawURLEncoding.EncodeToString(sum[:]) != f.challenge {
		status := f.status
		if status == 0 {
			status = http.StatusBadRequest
		}
		w.WriteHeader(status)
		w.Write([]byte(`{"error":"invalid_grant","error_description":"code expired or verifier mismatch"}`))
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"key": f.key, "access_token": f.key, "token_type": "Bearer"})
}

// browser returns an Open function that plays the user: it reads the sign-in URL, then
// calls the callback with query, filled in from the URL's state unless query sets its own.
func browser(t *testing.T, f *fakeNanoGPT, opened *url.URL, query url.Values) func(string) error {
	return func(raw string) error {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		*opened = *u
		f.challenge = u.Query().Get("code_challenge")
		if !query.Has("state") {
			query.Set("state", u.Query().Get("state"))
		}
		resp, err := http.Get(u.Query().Get("callback_url") + "?" + query.Encode())
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return nil
	}
}

func newOAuth(token string, open func(string) error, stderr *bytes.Buffer) OAuth {
	return OAuth{
		AuthURL:  "https://nano-gpt.example/auth",
		TokenURL: token,
		Timeout:  5 * time.Second,
		Client:   http.DefaultClient,
		Open:     open,
		Stderr:   stderr,
	}
}

func TestSignInReturnsTheExchangedKey(t *testing.T) {
	f := &fakeNanoGPT{code: "c1", key: "sk-nano-test"}
	srv := httptest.NewServer(f)
	defer srv.Close()
	var opened url.URL
	var stderr bytes.Buffer
	key, err := newOAuth(srv.URL, browser(t, f, &opened, url.Values{"code": {"c1"}}), &stderr).SignIn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if key != "sk-nano-test" {
		t.Errorf("key = %q", key)
	}
	q := opened.Query()
	if q.Get("code_challenge_method") != "S256" {
		t.Errorf("code_challenge_method = %q", q.Get("code_challenge_method"))
	}
	if !strings.Contains(q.Get("scope"), "api.use") {
		t.Errorf("scope = %q lacks api.use", q.Get("scope"))
	}
	if q.Get("client_name") != "rea-launcher" {
		t.Errorf("client_name = %q", q.Get("client_name"))
	}
	if cb, _ := url.Parse(q.Get("callback_url")); cb == nil || cb.Hostname() != "127.0.0.1" || cb.Port() == "" {
		t.Errorf("callback_url = %q, want a loopback address with a port", q.Get("callback_url"))
	}
	if len(q.Get("state")) < 32 {
		t.Errorf("state %q is too short to be unguessable", q.Get("state"))
	}
	if !strings.Contains(stderr.String(), opened.String()) {
		t.Errorf("stderr does not print the sign-in URL:\n%s", stderr.String())
	}
	if strings.Contains(stderr.String(), "sk-nano-test") {
		t.Error("stderr shows the key")
	}
}

func TestForeignCallbacksAreRefusedAndTheSignInWaits(t *testing.T) {
	f := &fakeNanoGPT{code: "c1", key: "sk-nano-test"}
	srv := httptest.NewServer(f)
	defer srv.Close()
	var opened url.URL
	var stderr bytes.Buffer
	genuine := browser(t, f, &opened, url.Values{"code": {"c1"}})
	open := func(raw string) error {
		u, _ := url.Parse(raw)
		cb := u.Query().Get("callback_url")
		state := u.Query().Get("state")
		forged := []*http.Request{}
		r, _ := http.NewRequest(http.MethodGet, cb+"?code=evil&state=forged", nil)
		forged = append(forged, r)
		r, _ = http.NewRequest(http.MethodGet, cb+"?error=access_denied&state=", nil)
		forged = append(forged, r)
		r, _ = http.NewRequest(http.MethodGet, cb+"?code=evil&state="+state, nil)
		r.Host = "rebound.example" // DNS rebinding reaches the port under another name
		forged = append(forged, r)
		r, _ = http.NewRequest(http.MethodPost, cb+"?code=evil&state="+state, nil)
		forged = append(forged, r)
		for _, req := range forged {
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("%s %s (Host %s): HTTP %d, want 400", req.Method, req.URL, req.Host, resp.StatusCode)
			}
		}
		return genuine(raw)
	}
	key, err := newOAuth(srv.URL, open, &stderr).SignIn(t.Context())
	if err != nil || key != "sk-nano-test" {
		t.Fatalf("key %q, err %v: a foreign callback ended the sign-in", key, err)
	}
	if f.calls.Load() != 1 {
		t.Errorf("%d exchanges, want only the genuine code's", f.calls.Load())
	}
}

func TestOnlyForeignCallbacksTimeOutWithoutAnExchange(t *testing.T) {
	f := &fakeNanoGPT{code: "c1", key: "sk-nano-test"}
	srv := httptest.NewServer(f)
	defer srv.Close()
	var opened url.URL
	var stderr bytes.Buffer
	o := newOAuth(srv.URL, browser(t, f, &opened, url.Values{"code": {"c1"}, "state": {"forged"}}), &stderr)
	o.Timeout = 200 * time.Millisecond
	if _, err := o.SignIn(t.Context()); err == nil {
		t.Fatal("want a timeout")
	}
	if f.calls.Load() != 0 {
		t.Error("a forged callback's code was exchanged")
	}
}

func TestSignInReportsDenial(t *testing.T) {
	f := &fakeNanoGPT{code: "c1", key: "sk-nano-test"}
	srv := httptest.NewServer(f)
	defer srv.Close()
	var opened url.URL
	var stderr bytes.Buffer
	_, err := newOAuth(srv.URL, browser(t, f, &opened, url.Values{"error": {"access_denied"}}), &stderr).SignIn(t.Context())
	if err == nil || !strings.Contains(err.Error(), "access_denied") {
		t.Fatalf("err = %v, want access_denied", err)
	}
}

func TestSignInFailsWhenTheExchangeFails(t *testing.T) {
	f := &fakeNanoGPT{code: "c1", key: "sk-nano-test", status: http.StatusBadRequest}
	srv := httptest.NewServer(f)
	defer srv.Close()
	var opened url.URL
	var stderr bytes.Buffer
	_, err := newOAuth(srv.URL, browser(t, f, &opened, url.Values{"code": {"c1"}}), &stderr).SignIn(t.Context())
	if err == nil || !strings.Contains(err.Error(), "invalid_grant") {
		t.Fatalf("err = %v, want invalid_grant", err)
	}
}

func TestSignInTimesOut(t *testing.T) {
	var stderr bytes.Buffer
	o := newOAuth("http://127.0.0.1:1", func(string) error { return nil }, &stderr)
	o.Timeout = 50 * time.Millisecond
	if _, err := o.SignIn(t.Context()); err == nil {
		t.Fatal("want an error when no callback arrives")
	}
}

func TestSignInWorksWhenNoBrowserOpens(t *testing.T) {
	f := &fakeNanoGPT{code: "c1", key: "sk-nano-test"}
	srv := httptest.NewServer(f)
	defer srv.Close()
	var opened url.URL
	var stderr bytes.Buffer
	byHand := browser(t, f, &opened, url.Values{"code": {"c1"}})
	open := func(raw string) error {
		go byHand(raw)
		return errors.New("no browser")
	}
	key, err := newOAuth(srv.URL, open, &stderr).SignIn(t.Context())
	if err != nil || key != "sk-nano-test" {
		t.Fatalf("key %q, err %v", key, err)
	}
	if !strings.Contains(stderr.String(), "by hand") {
		t.Errorf("stderr does not say to open the URL by hand:\n%s", stderr.String())
	}
}

func callbackPort(t *testing.T, opened url.URL) string {
	t.Helper()
	cb, err := url.Parse(opened.Query().Get("callback_url"))
	if err != nil {
		t.Fatal(err)
	}
	return cb.Port()
}

func TestTheSignInUsesItsFixedPortAndFallsBackWhenBusy(t *testing.T) {
	f := &fakeNanoGPT{code: "c1", key: "sk-nano-test"}
	srv := httptest.NewServer(f)
	defer srv.Close()

	free, _ := net.Listen("tcp", "127.0.0.1:0")
	port := free.Addr().(*net.TCPAddr).Port
	free.Close()
	var opened url.URL
	var stderr bytes.Buffer
	o := newOAuth(srv.URL, browser(t, f, &opened, url.Values{"code": {"c1"}}), &stderr)
	o.Port = port
	if _, err := o.SignIn(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := callbackPort(t, opened); got != strconv.Itoa(port) {
		t.Errorf("callback on port %s, want the fixed %d", got, port)
	}

	busy, _ := net.Listen("tcp", "127.0.0.1:0")
	defer busy.Close()
	o.Port = busy.Addr().(*net.TCPAddr).Port
	o.Open = browser(t, f, &opened, url.Values{"code": {"c1"}})
	stderr.Reset()
	if _, err := o.SignIn(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := callbackPort(t, opened); got == strconv.Itoa(o.Port) || got == "" {
		t.Errorf("callback on port %s, want another than the busy %d", got, o.Port)
	}
	if !strings.Contains(stderr.String(), "busy") {
		t.Errorf("stderr does not say the port is busy:\n%s", stderr.String())
	}
}
