package module

import (
	"context"
	"errors"
	"io"
)

type Blob interface {
	Sync() error
	io.Writer
	io.Closer
}

var ErrNoSuchBlob = errors.New("blob_store: no such object")

const UnknownBlobSize int64 = -1

// BlobStore is the interface used by modules providing large binary object
// storage.
type BlobStore interface {
	// Create creates a new blob for writing.
	//
	// Sync will be called on the returned Blob object after -all- data has
	// been successfully written.
	//
	// Close without Sync can be assumed to happen due to an unrelated error
	// and stored data can be discarded.
	//
	// blobSize indicates the exact amount of bytes that will be written
	// If -1 is passed - it is unknown and implementation will not make
	// any assumptions about the blob size. Error can be returned by any
	// Blob method if more than than blobSize bytes get written.
	//
	// Passed context will cover the entire blob write operation.
	Create(ctx context.Context, key string, blobSize int64) (Blob, error)

	// Open returns the reader for the object specified by
	// passed key.
	//
	// If no such object exists - ErrNoSuchBlob is returned.
	Open(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes a set of keys from store. Non-existent keys are ignored.
	Delete(ctx context.Context, keys []string) error
}
