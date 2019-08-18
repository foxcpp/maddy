// The buffer package provides utilities for temporary storage (buffering)
// of large blobs.
package buffer

import (
	"io"
)

// Buffer interface represents abstract temporary storage for blobs.
//
// The Buffer storage is assumed to be immutable. If any modifications
// are made - new storage location should be used for them.
// This is important to ensure goroutine-safety.
//
// Since Buffer objects require a careful management of lifetimes, here
// is the convention: Its always creator responsibility to call Remove after
// Buffer is no longer used. If Buffer object is passed to a function - it is not
// guaranteed to be valid after this function returns. If function needs to preserve
// the storage contents, it should "re-buffer" it either by reading entire blob
// and storing it somewhere or applying implementation-specific methods (for example,
// the FileBuffer storage may be "re-buffered" by hard-linking the underlying file).
type Buffer interface {
	// Open creates new Reader reading from the underlying storage.
	Open() (io.ReadCloser, error)

	// Remove discards buffered body and releases all associated resources.
	//
	// Multiple Buffer objects may refer to the same underlying storage.
	// In this case, care should be taken to ensure that Remove is called
	// only once since it will discard the shared storage and invalidate
	// all Buffer objects using it.
	//
	// Readers previously created using Open can still be used, but
	// new ones can't be created.
	Remove() error
}
