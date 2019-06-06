package maddy

import (
	"errors"
	"fmt"
	"net/mail"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/emersion/maddy/log"
	"github.com/emersion/maddy/module"
	"github.com/google/uuid"
)

func SubmissionPrepare(ctx *module.DeliveryContext) error {
	ctx.DontTraceSender = true

	if ctx.Header.Get("Message-ID") == "" {
		msgId, err := uuid.NewRandom()
		if err != nil {
			return errors.New("Message-ID generation failed")
		}
		log.Debugf("adding missing Message-ID header to message from %s (%s)", ctx.SrcHostname, ctx.SrcAddr)
		ctx.Header.Set("Message-ID", "<"+msgId.String()+"@"+ctx.OurHostname+">")
	}

	if ctx.Header.Get("From") == "" {
		return &smtp.SMTPError{
			Code:    554,
			Message: "Message does not contains a From header field",
		}
	}

	for _, hdr := range [...]string{"Sender"} {
		if value := ctx.Header.Get(hdr); value != "" {
			if _, err := mail.ParseAddress(value); err != nil {
				return &smtp.SMTPError{
					Code:    554,
					Message: fmt.Sprintf("Invalid address in %s: %v", hdr, err),
				}
			}
		}
	}
	for _, hdr := range [...]string{"To", "Cc", "Bcc", "Reply-To"} {
		if value := ctx.Header.Get(hdr); value != "" {
			if _, err := mail.ParseAddressList(value); err != nil {
				return &smtp.SMTPError{
					Code:    554,
					Message: fmt.Sprintf("Invalid address in %s: %v", hdr, err),
				}
			}
		}
	}

	addrs, err := mail.ParseAddressList(ctx.Header.Get("From"))
	if err != nil {
		return &smtp.SMTPError{
			Code:    554,
			Message: fmt.Sprintf("Invalid address in From: %v", err),
		}
	}

	// https://tools.ietf.org/html/rfc5322#section-3.6.2
	// If From contains multiple addresses, Sender field must be present.
	if len(addrs) > 1 && ctx.Header.Get("Sender") == "" {
		return &smtp.SMTPError{
			Code:    554,
			Message: "Missing Sender header field",
		}
	}

	if dateHdr := ctx.Header.Get("Date"); dateHdr != "" {
		_, err := time.Parse("Mon, 2 Jan 2006 15:04:05 -0700", dateHdr)
		if err != nil {
			return &smtp.SMTPError{
				Code:    554,
				Message: "Malformed Data header",
			}
		}
	} else {
		log.Debugf("adding missing Date header to message from %s (%s)", ctx.SrcHostname, ctx.SrcAddr)
		ctx.Header.Set("Date", time.Now().Format("Mon, 2 Jan 2006 15:04:05 -0700"))
	}

	return nil
}
