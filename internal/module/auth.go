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
	AuthPlain(username, password string) ([]string, error)
}

// SASLProvider is the interface implemented by modules and used by protocol
// endpoints that rely on SASL framework for user authentication.
//
// This actual interface is only used to indicate that the module is a
// SASL-compatible auth. provider. For each unique value returned by
// SASLMechanisms, the module object should also implement the coresponding
// mechanism-specific interface.
//
// *Rationale*: There is no single generic interface that would handle any SASL
// mechanism while permiting the use of a credentials set estabilished once with
// multiple auth. providers at once.
//
// Per-mechanism interfaces:
// - PLAIN => PlainAuth
type SASLProvider interface {
	SASLMechanisms() []string
}
