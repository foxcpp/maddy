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
	MessageBodySize        = 100 * 1024
	MessageHeaderFields    = 25
	MessageHeaderFieldSize = 100
)

type multipleErrs map[string]error

func (m multipleErrs) SetStatus(rcptTo string, err error) {
	m[rcptTo] = err
}

func RandomMsg(b *testing.B) (module.MsgMetadata, textproto.Header, buffer.Buffer) {
	IDRaw := sha1.Sum([]byte(b.Name()))
	encodedID := hex.EncodeToString(IDRaw[:])

	hdr := textproto.Header{}
	for i := 0; i < MessageHeaderFields; i++ {
		hdr.Add("AAAAAAAAAAAA-"+strconv.Itoa(i), strings.Repeat("A", MessageHeaderFieldSize))
	}

	return module.MsgMetadata{
		DontTraceSender: true,
		ID:              encodedID,
	}, hdr, buffer.MemoryBuffer{Slice: bytes.Repeat([]byte("a"), MessageBodySize)}
}

func BenchDelivery(b *testing.B, target module.DeliveryTarget, sender string, recipientTemplates []string) {
	b.Run("Start", func(b *testing.B) {
		meta, _, _ := RandomMsg(b)

		deliveries := make([]module.Delivery, 0, b.N)

		for i := 0; i < b.N; i++ {
			delivery, err := target.Start(&meta, sender)
			if err != nil {
				b.Fatal(err)
			}
			deliveries = append(deliveries, delivery)
		}

		// Kept outside of main loop to avoid measuring it too.
		for _, delivery := range deliveries {
			delivery.Abort()
		}
	})

	b.Run("AddRcpt", func(b *testing.B) {
		meta, _, _ := RandomMsg(b)
		delivery, err := target.Start(&meta, sender)
		if err != nil {
			b.Fatal(err)
		}
		defer delivery.Abort()

		for i := 0; i < b.N; i++ {
			rcpt := strings.Replace(recipientTemplates[i%len(recipientTemplates)], "X", strconv.Itoa(i), -1)

			if err := delivery.AddRcpt(rcpt); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Body", func(b *testing.B) {
		meta, header, body := RandomMsg(b)
		delivery, err := target.Start(&meta, sender)
		if err != nil {
			b.Fatal(err)
		}
		defer delivery.Abort()

		for i, rcptTemplate := range recipientTemplates {
			rcpt := strings.Replace(rcptTemplate, "X", strconv.Itoa(i), -1)

			if err := delivery.AddRcpt(rcpt); err != nil {
				b.Fatal(err)
			}
		}

		// XXX: This relies on DeliveryTarget having an idempotent Body implementation.
		for i := 0; i < b.N; i++ {
			if err := delivery.Body(header, body); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("BodyNonAtomic", func(b *testing.B) {
		meta, header, body := RandomMsg(b)
		delivery, err := target.Start(&meta, sender)
		if err != nil {
			b.Fatal(err)
		}
		defer delivery.Abort()

		partialDelivery, ok := delivery.(module.PartialDelivery)
		if !ok {
			b.Skip("Delivery does not implement PartialDelivery")
		}

		for i, rcptTemplate := range recipientTemplates {
			rcpt := strings.Replace(rcptTemplate, "X", strconv.Itoa(i), -1)

			if err := delivery.AddRcpt(rcpt); err != nil {
				b.Fatal(err)
			}
		}

		sc := multipleErrs{}

		// XXX: This relies on DeliveryTarget having an idempotent BodyNonAtomic implementation.
		for i := 0; i < b.N; i++ {
			partialDelivery.BodyNonAtomic(sc, header, body)
		}
	})

	b.Run("Full transaction", func(b *testing.B) {
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
	})
}
