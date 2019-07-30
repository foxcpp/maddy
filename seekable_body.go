package maddy

import (
	"io"
	"io/ioutil"

	"github.com/emersion/maddy/log"
	"github.com/emersion/maddy/module"
)

func seekableBody(body io.Reader) (io.ReadSeeker, error) {
	if seeker, ok := body.(io.ReadSeeker); ok {
		return seeker, nil
	}
	if byter, ok := body.(module.Byter); ok {
		return module.NewBytesReader(byter.Bytes()), nil
	}

	bodyBlob, err := ioutil.ReadAll(body)
	log.Debugln("message body copied or buffered, size:", len(bodyBlob))
	return module.NewBytesReader(bodyBlob), err
}
