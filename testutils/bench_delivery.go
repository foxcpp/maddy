package testutils

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/module"
)

// Empirically observed "around average" values.
const (
	MessageBodySize             = 100 * 1024
	ExtraMessageHeaderFields    = 20
	ExtraMessageHeaderFieldSize = 100
)

var headerPreamble = map[string]string{
	"From":                      `"whatever whatever" <whatever@example.org>`,
	"Message-ID":                `<AAAAAAAAAAAAAAAAAA@example.org>`,
	"To":                        `"whatever whatever" <whatever@example.org>, "fool" <fool@example.org>`,
	"Date":                      "Tue, 08 Oct 2019 06:25:41 +0000",
	"Subject":                   "Heya Heya Heya Heya",
	"Content-Type":              "text/plain; charset=us-ascii",
	"MIME-Version":              "1.0",
	"Content-Transfer-Encoding": "8bit",
	"Received":                  `from whatever ([127.0.0.1]) by whatever ([127.0.0.1]) with whatever id whatever for whatever@example.org`,
}

type multipleErrs map[string]error

func (m multipleErrs) SetStatus(rcptTo string, err error) {
	m[rcptTo] = err
}

func RandomMsg(b *testing.B) (module.MsgMetadata, textproto.Header, buffer.Buffer) {
	IDRaw := sha1.Sum([]byte(b.Name()))
	encodedID := hex.EncodeToString(IDRaw[:])

	hdr := textproto.Header{}

	for k, v := range headerPreamble {
		hdr.Add(k, v)
	}

	for i := 0; i < ExtraMessageHeaderFields; i++ {
		hdr.Add("AAAAAAAAAAAA-"+strconv.Itoa(i), strings.Repeat("A", ExtraMessageHeaderFieldSize))
	}

	return module.MsgMetadata{
		DontTraceSender: true,
		ID:              encodedID,
	}, hdr, buffer.MemoryBuffer{Slice: bytes.Repeat([]byte("a"), MessageBodySize)}
}

func BenchDelivery(b *testing.B, target module.DeliveryTarget, sender string, recipientTemplates []string) {
	meta, header, body := RandomMsg(b)

	for i := 0; i < b.N; i++ {
		delivery, err := target.Start(&meta, sender)
		if err != nil {
			b.Fatal(err)
		}

		for i, rcptTemplate := range recipientTemplates {
			rcpt := strings.Replace(rcptTemplate, "X", strconv.Itoa(i), -1)

			if err := delivery.AddRcpt(rcpt); err != nil {
				b.Fatal(err)
			}
		}

		if err := delivery.Body(header, body); err != nil {
			b.Fatal(err)
		}

		if err := delivery.Commit(); err != nil {
			b.Fatal(err)
		}
	}
}
