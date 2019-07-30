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

// BodyBuffer interface represents abstract temporarly storage for the message
// body.
type BodyBuffer interface {
	// Open creates new Reader reading from the underlying storage.
	Open() (io.ReadCloser, error)

	// Close discards buffered body and releases all associated resources.
	//
	// All Readers created using Open should be closed before calling this
	// function.
	Close() error
}

// MemoryBuffer implements BodyBuffer interface using byte slice.
type MemoryBuffer struct {
	Slice []byte
}

func (mb MemoryBuffer) Open() (io.ReadCloser, error) {
	return NewBytesReader(mb.Slice), nil
}

func (mb MemoryBuffer) Close() error {
	return nil
}

// BufferInMemory is a convenience function which creates MemoryBuffer with
// contents of the passed io.Reader.
func BufferInMemory(r io.Reader) (BodyBuffer, error) {
	blob, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return MemoryBuffer{Slice: blob}, nil
}

// FileBuffer implements BodyBuffer interface using file system.
type FileBuffer struct {
	Path string
}

func (fb FileBuffer) Open() (io.ReadCloser, error) {
	return os.Open(fb.Path)
}

func (fb FileBuffer) Close() error {
	return os.Remove(fb.Path)
}

// BufferInFile is a convenience function which creates FileBuffer with underlying
// file created in the specified directory with the random name.
func BufferInFile(r io.Reader, dir string) (BodyBuffer, error) {
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
