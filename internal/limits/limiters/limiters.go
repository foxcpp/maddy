// Package limiters provides a set of wrappers intended to restrict the amount
// of resources consumed by the server.
package limiters

import "context"

// The L interface represents a blocking limiter that has some upper bound of
// resource use and blocks when it is exceeded until enough resources are
// freed.
type L interface {
	Take() bool
	TakeContext(context.Context) error
	Release()

	// Close frees any resources used internally by Limiter for book-keeping.
	Close()
}
