/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2025 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

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

package target

import "io"

// SourceReader wraps the body passed to module.StreamingDelivery.BodyStream
// and records the error returned by the message source, so that the target
// can tell it apart from its own errors.
type SourceReader struct {
	r   io.Reader
	err error
}

func NewSourceReader(r io.Reader) *SourceReader {
	return &SourceReader{r: r}
}

func (r *SourceReader) Read(b []byte) (int, error) {
	n, err := r.r.Read(b)
	if err != nil && err != io.EOF {
		r.err = err
	}
	return n, err
}

// Err returns the last error other than io.EOF returned by the underlying
// reader.
func (r *SourceReader) Err() error {
	return r.err
}
