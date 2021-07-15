package module

import (
	"errors"
	"io"
)

type Blob interface {
	Sync() error
	io.Writer
	io.Closer
}

var ErrNoSuchBlob = errors.New("blob_store: no such object")

// BlobStore is the interface used by modules providing large binary object
// storage.
type BlobStore interface {
	Create(key string) (Blob, error)

	// Open returns the reader for the object specified by
	// passed key.
	//
	// If no such object exists - ErrNoSuchBlob is returned.
	Open(key string) (io.ReadCloser, error)

	// Delete removes a set of keys from store. Non-existent keys are ignored.
	Delete(keys []string) error
}
