package maddy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"time"

	"github.com/emersion/go-message"
	"github.com/emersion/go-smtp"
	"github.com/emersion/maddy/log"
	"github.com/emersion/maddy/module"
	"github.com/google/uuid"
)

type submissionPrepareStep struct{}

func (step submissionPrepareStep) Pass(ctx *module.DeliveryContext, msg io.Reader) (io.Reader, bool, error) {
	parsed, err := message.Read(msg)
	if err != nil {
		return nil, false, errors.New("Message can't be parsed")
	}

	if parsed.Header.Get("Message-ID") == "" {
		msgId, err := uuid.NewRandom()
		if err != nil {
			return nil, false, errors.New("Message-ID generation failed")
		}
		log.Debugf("adding missing Message-ID header to message from %s (%s)", ctx.SrcHostname, ctx.SrcAddr)
		parsed.Header.Set("Message-ID", "<"+msgId.String()+"@"+ctx.OurHostname+">")
	}

	if parsed.Header.Get("From") == "" {
		return nil, false, &smtp.SMTPError{
			Code:    554,
			Message: "Message does not contains a From header field",
		}
	}

	for _, hdr := range [...]string{"Sender"} {
		if value := parsed.Header.Get(hdr); value != "" {
			if _, err := mail.ParseAddress(value); err != nil {
				return nil, false, &smtp.SMTPError{
					Code:    554,
					Message: fmt.Sprintf("Invalid address in %s: %v", hdr, err),
				}
			}
		}
	}
	for _, hdr := range [...]string{"To", "Cc", "Bcc", "Reply-To"} {
		if value := parsed.Header.Get(hdr); value != "" {
			if _, err := mail.ParseAddressList(value); err != nil {
				return nil, false, &smtp.SMTPError{
					Code:    554,
					Message: fmt.Sprintf("Invalid address in %s: %v", hdr, err),
				}
			}
		}
	}

	addrs, err := mail.ParseAddressList(parsed.Header.Get("From"))
	if err != nil {
		return nil, false, &smtp.SMTPError{
			Code:    554,
			Message: fmt.Sprintf("Invalid address in From: %v", err),
		}
	}

	// https://tools.ietf.org/html/rfc5322#section-3.6.2
	// If From contains multiple addresses, Sender field must be present.
	if len(addrs) > 1 && parsed.Header.Get("Sender") == "" {
		return nil, false, &smtp.SMTPError{
			Code:    554,
			Message: "Missing Sender header field",
		}
	}

	if dateHdr := parsed.Header.Get("Date"); dateHdr != "" {
		_, err := time.Parse("Mon, 2 Jan 2006 15:04:05 -0700", dateHdr)
		if err != nil {
			return nil, false, &smtp.SMTPError{
				Code:    554,
				Message: "Malformed Data header",
			}
		}
	} else {
		log.Debugf("adding missing Date header to message from %s (%s)", ctx.SrcHostname, ctx.SrcAddr)
		parsed.Header.Set("Date", time.Now().Format("Mon, 2 Jan 2006 15:04:05 -0700"))
	}

	if ctx.From != "" && parsed.Header.Get("Return-Path") == "" {
		parsed.Header.Set("Return-Path", ctx.From)
	}

	var buf bytes.Buffer
	if err := parsed.WriteTo(&buf); err != nil {
		return nil, false, err
	}

	return &buf, true, nil
}
