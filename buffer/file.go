package buffer

import (
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
)

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
