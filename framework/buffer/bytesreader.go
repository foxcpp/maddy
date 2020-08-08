/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package buffer

import (
	"bytes"
)

// BytesReader is a wrapper for bytes.Reader that stores the original []byte
// value and allows to retrieve it.
//
// It is meant for passing to libraries that expect a io.Reader
// but apply certain optimizations when the Reader implements
// Bytes() interface.
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
