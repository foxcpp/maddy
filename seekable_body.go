package maddy

import (
	"io"
	"io/ioutil"

	"github.com/emersion/maddy/buffer"
	"github.com/emersion/maddy/log"
)

func seekableBody(body io.Reader) (io.ReadSeeker, error) {
	if seeker, ok := body.(io.ReadSeeker); ok {
		return seeker, nil
	}
	if byter, ok := body.(buffer.Byter); ok {
		return buffer.NewBytesReader(byter.Bytes()), nil
	}

	bodyBlob, err := ioutil.ReadAll(body)
	log.Debugln("message body copied or buffered, size:", len(bodyBlob))
	return buffer.NewBytesReader(bodyBlob), err
}
