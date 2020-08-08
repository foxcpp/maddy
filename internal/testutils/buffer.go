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

package testutils

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/framework/buffer"
)

func BodyFromStr(t *testing.T, literal string) (textproto.Header, buffer.MemoryBuffer) {
	t.Helper()

	bufr := bufio.NewReader(strings.NewReader(literal))
	hdr, err := textproto.ReadHeader(bufr)
	if err != nil {
		t.Fatal(err)
	}
	body, err := ioutil.ReadAll(bufr)
	if err != nil {
		t.Fatal(err)
	}

	return hdr, buffer.MemoryBuffer{Slice: body}
}

type errorReader struct {
	r   io.Reader
	err error
}

func (r *errorReader) Read(b []byte) (int, error) {
	n, err := r.r.Read(b)
	if err == io.EOF {
		return n, r.err
	}
	return n, err
}

type FailingBuffer struct {
	Blob []byte

	OpenError error
	IOError   error
}

func (fb FailingBuffer) Open() (io.ReadCloser, error) {
	r := ioutil.NopCloser(bytes.NewReader(fb.Blob))

	if fb.IOError != nil {
		return ioutil.NopCloser(&errorReader{r, fb.IOError}), fb.OpenError
	}

	return r, fb.OpenError
}

func (fb FailingBuffer) Len() int {
	return len(fb.Blob)
}

func (fb FailingBuffer) Remove() error {
	return nil
}
