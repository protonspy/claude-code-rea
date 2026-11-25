package launcher

import (
	"context"
	"errors"
	"net/http"
)

// Artifact is a pinned download.
type Artifact struct {
	Name      string // what it is, for messages
	File      string // the file name it is saved under
	URL       string
	Integrity string // an SRI hash: "sha256-<base64>" or "sha512-<base64>"
	Format    string // formatZip or formatTarGz; empty for an archive npm installs
	Strip     int    // leading path components dropped when unpacking
}

// fetch downloads a into dir and returns its path once its content matches a.Integrity.
func fetch(ctx context.Context, client *http.Client, a Artifact, dir string) (string, error) {
	return "", errors.New("not implemented")
}

// publish installs dest by filling a temporary directory beside it and renaming that into
// place. An existing dest is an installed component and is left alone.
func publish(dest string, fill func(tmp string) error) error {
	return errors.New("not implemented")
}
