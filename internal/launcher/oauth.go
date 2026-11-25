package launcher

import (
	"context"
	"errors"
	"io"
	"net/http"
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

// SignIn runs the sign-in and returns the key NanoGPT issues.
func (o OAuth) SignIn(ctx context.Context) (string, error) {
	return "", errors.New("not implemented")
}
