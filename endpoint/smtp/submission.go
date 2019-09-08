package smtp

import (
	"errors"
	"fmt"
	"net/mail"
	"time"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/module"
	"github.com/google/uuid"
)

func SubmissionPrepare(msgMeta *module.MsgMetadata, header textproto.Header) error {
	msgMeta.DontTraceSender = true

	if header.Get("Message-ID") == "" {
		msgId, err := uuid.NewRandom()
		if err != nil {
			return errors.New("Message-ID generation failed")
		}
		// TODO: Move this function to smtpSession and use its logger.
		log.Debugf("adding missing Message-ID header to message from %s (%s)", msgMeta.SrcHostname, msgMeta.SrcAddr)
		header.Set("Message-ID", "<"+msgId.String()+"@"+msgMeta.OurHostname+">")
	}

	if header.Get("From") == "" {
		return &smtp.SMTPError{
			Code:    554,
			Message: "Message does not contains a From header field",
		}
	}

	for _, hdr := range [...]string{"Sender"} {
		if value := header.Get(hdr); value != "" {
			if _, err := mail.ParseAddress(value); err != nil {
				return &smtp.SMTPError{
					Code:    554,
					Message: fmt.Sprintf("Invalid address in %s: %v", hdr, err),
				}
			}
		}
	}
	for _, hdr := range [...]string{"To", "Cc", "Bcc", "Reply-To"} {
		if value := header.Get(hdr); value != "" {
			if _, err := mail.ParseAddressList(value); err != nil {
				return &smtp.SMTPError{
					Code:    554,
					Message: fmt.Sprintf("Invalid address in %s: %v", hdr, err),
				}
			}
		}
	}

	addrs, err := mail.ParseAddressList(header.Get("From"))
	if err != nil {
		return &smtp.SMTPError{
			Code:    554,
			Message: fmt.Sprintf("Invalid address in From: %v", err),
		}
	}

	// https://tools.ietf.org/html/rfc5322#section-3.6.2
	// If From contains multiple addresses, Sender field must be present.
	if len(addrs) > 1 && header.Get("Sender") == "" {
		return &smtp.SMTPError{
			Code:    554,
			Message: "Missing Sender header field",
		}
	}

	if dateHdr := header.Get("Date"); dateHdr != "" {
		_, err := time.Parse("Mon, 2 Jan 2006 15:04:05 -0700", dateHdr)
		if err != nil {
			return &smtp.SMTPError{
				Code:    554,
				Message: "Malformed Data header",
			}
		}
	} else {
		log.Debugf("adding missing Date header to message from %s (%s)", msgMeta.SrcHostname, msgMeta.SrcAddr)
		header.Set("Date", time.Now().Format("Mon, 2 Jan 2006 15:04:05 -0700"))
	}

	return nil
}
