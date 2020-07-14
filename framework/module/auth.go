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
//
// Modules implementing this interface should be registered with "auth." prefix in name.
type PlainAuth interface {
	AuthPlain(username, password string) error
}

// PlainUserDB is a local credentials store that can be managed using maddyctl
// utility.
type PlainUserDB interface {
	PlainAuth
	ListUsers() ([]string, error)
	CreateUser(username, password string) error
	SetUserPassword(username, password string) error
	DeleteUser(username string) error
}
