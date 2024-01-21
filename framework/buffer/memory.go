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
	"io"
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
	blob, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return MemoryBuffer{Slice: blob}, nil
}
