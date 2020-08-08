//+build !hz_gb_2312

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

package maddy

import (
	"errors"

	"github.com/emersion/go-message/charset"
	"golang.org/x/text/encoding"
)

/*

Disallow hz-gb-2312 encoding as it can trigger OOM crash.

https://github.com/emersion/go-message/issues/95
https://github.com/golang/go/issues/35118
*/

type dummyEncoding struct{}

func (dummyEncoding) NewDecoder() *encoding.Decoder {
	return &encoding.Decoder{Transformer: dummyEncoding{}}
}
func (dummyEncoding) NewEncoder() *encoding.Encoder {
	return &encoding.Encoder{Transformer: dummyEncoding{}}
}

func (dummyEncoding) Reset() {}

func (dummyEncoding) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	return 0, 0, errors.New("hz-gb-2312 decoding is disabled due to known issues")
}

func init() {
	charset.RegisterEncoding("hz-gb-2312", dummyEncoding{})
}
