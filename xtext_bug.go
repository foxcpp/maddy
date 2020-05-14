//+build !hz_gb_2312

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
