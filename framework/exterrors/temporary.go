package exterrors

import (
	"errors"
)

type TemporaryErr interface {
	Temporary() bool
}

// IsTemporaryOrUnspec is similar to IsTemporary except that it returns true
// if error does not have a Temporary() method. Basically, it assumes that
// errors are temporary by default compared to IsTemporary that assumes
// errors are permanent by default.
func IsTemporaryOrUnspec(err error) bool {
	var temp TemporaryErr
	if errors.As(err, &temp) {
		return temp.Temporary()
	}
	return true
}

// IsTemporary returns true whether the passed error object
// have a Temporary() method and it returns true.
func IsTemporary(err error) bool {
	var temp TemporaryErr
	if errors.As(err, &temp) {
		return temp.Temporary()
	}
	return false
}

type temporaryErr struct {
	err  error
	temp bool
}

func (t temporaryErr) Unwrap() error {
	return t.err
}

func (t temporaryErr) Error() string {
	return t.err.Error()
}

func (t temporaryErr) Temporary() bool {
	return t.temp
}

// WithTemporary wraps the passed error object with the implementation of the
// Temporary() method that will return the specified value.
//
// Original error value can be obtained using errors.Unwrap.
func WithTemporary(err error, temporary bool) error {
	return temporaryErr{err, temporary}
}
