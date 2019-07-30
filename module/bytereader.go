package module

import "bytes"

type Byter interface {
	Bytes() []byte
}

// BytesReader is a wrapper for bytes.Reader that stores the original []byte
// value and allows to retrieve it.
type BytesReader struct {
	*bytes.Reader
	value []byte
}

// Bytes returns the unread portion of underlying slice used to construct
// BytesReader.
func (br BytesReader) Bytes() []byte {
	return br.value[int(br.Size())-br.Len():]
}

// Copy returns the BytesReader reading from the same slice as br at the same
// position.
func (br BytesReader) Copy() BytesReader {
	return NewBytesReader(br.Bytes())
}

// Close is a dummy method for implementation of io.Closer so BytesReader can
// be used in MemoryBuffer directly.
func (br BytesReader) Close() error {
	return nil
}

func NewBytesReader(b []byte) BytesReader {
	// BytesReader and not *BytesReader because BytesReader already wraps two
	// pointers and double indirection would be pointless.
	return BytesReader{
		Reader: bytes.NewReader(b),
		value:  b,
	}
}
