package module

import (
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
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

// MemoryBuffer implements Buffer interface using byte slice.
type MemoryBuffer struct {
	Slice []byte
}

func (mb MemoryBuffer) Open() (io.ReadCloser, error) {
	return NewBytesReader(mb.Slice), nil
}

func (mb MemoryBuffer) Remove() error {
	return nil
}

// BufferInMemory is a convenience function which creates MemoryBuffer with
// contents of the passed io.Reader.
func BufferInMemory(r io.Reader) (Buffer, error) {
	blob, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return MemoryBuffer{Slice: blob}, nil
}

// FileBuffer implements Buffer interface using file system.
type FileBuffer struct {
	Path string
}

func (fb FileBuffer) Open() (io.ReadCloser, error) {
	return os.Open(fb.Path)
}

func (fb FileBuffer) Remove() error {
	return os.Remove(fb.Path)
}

// BufferInFile is a convenience function which creates FileBuffer with underlying
// file created in the specified directory with the random name.
func BufferInFile(r io.Reader, dir string) (Buffer, error) {
	// It is assumed that PRNG is initialized somewhere during program startup.
	nameBytes := make([]byte, 32)
	_, err := rand.Read(nameBytes)
	if err != nil {
		return nil, fmt.Errorf("buffer: failed to generate randomness for file name: %v", err)
	}
	path := filepath.Join(dir, hex.EncodeToString(nameBytes))
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("buffer: failed to create file: %v", err)
	}
	if _, err = io.Copy(f, r); err != nil {
		return nil, fmt.Errorf("buffer: failed to write file: %v", err)
	}
	// TODO: Consider caching an *os.File object for faster first Open().
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("buffer: failed to close file: %v", err)
	}

	return FileBuffer{Path: path}, nil
}
