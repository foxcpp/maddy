package module

import "errors"

var (
	// ErrUnknownCredentials should be returned by auth. provider if supplied
	// credentials are valid for it but are not recognized (e.g. not found in
	// used DB).
	ErrUnknownCredentials = errors.New("unknown credentials")
)

// PlainAuth is the interface implemented by modules providing authentication using
// username:password pairs.
type PlainAuth interface {
	AuthPlain(username, password string) error
}
