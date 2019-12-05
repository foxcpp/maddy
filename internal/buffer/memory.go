package buffer

import (
	"io"
	"io/ioutil"
)

// MemoryBuffer implements Buffer interface using byte slice.
type MemoryBuffer struct {
	Slice []byte
}

func (mb MemoryBuffer) Open() (io.ReadCloser, error) {
	return NewBytesReader(mb.Slice), nil
}

func (mb MemoryBuffer) Len() int {
	return len(mb.Slice)
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
