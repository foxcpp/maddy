package smtp

import (
	"errors"
	"fmt"
	"net/mail"
	"time"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/internal/exterrors"
	"github.com/foxcpp/maddy/internal/module"
	"github.com/google/uuid"
)

var (
	msgIDField = func() (string, error) {
		id, err := uuid.NewRandom()
		if err != nil {
			return "", err
		}
		return id.String(), nil
	}

	now = time.Now
)

func (s *Session) submissionPrepare(msgMeta *module.MsgMetadata, header *textproto.Header) error {
	msgMeta.DontTraceSender = true

	if header.Get("Message-ID") == "" {
		msgId, err := msgIDField()
		if err != nil {
			return errors.New("Message-ID generation failed")
		}
		s.log.Msg("adding missing Message-ID")
		header.Set("Message-ID", "<"+msgId+"@"+s.endp.serv.Domain+">")
	}

	if header.Get("From") == "" {
		return &exterrors.SMTPError{
			Code:         554,
			EnhancedCode: exterrors.EnhancedCode{5, 6, 0},
			Message:      "Message does not contains a From header field",
			Misc: map[string]interface{}{
				"modifier": "submission_prepare",
			},
		}
	}

	for _, hdr := range [...]string{"Sender"} {
		if value := header.Get(hdr); value != "" {
			if _, err := mail.ParseAddress(value); err != nil {
				return &exterrors.SMTPError{
					Code:         554,
					EnhancedCode: exterrors.EnhancedCode{5, 6, 0},
					Message:      fmt.Sprintf("Invalid address in %s", hdr),
					Misc: map[string]interface{}{
						"modifier": "submission_prepare",
						"addr":     value,
					},
					Err: err,
				}
			}
		}
	}
	for _, hdr := range [...]string{"To", "Cc", "Bcc", "Reply-To"} {
		if value := header.Get(hdr); value != "" {
			if _, err := mail.ParseAddressList(value); err != nil {
				return &exterrors.SMTPError{
					Code:         554,
					EnhancedCode: exterrors.EnhancedCode{5, 6, 0},
					Message:      fmt.Sprintf("Invalid address in %s", hdr),
					Misc: map[string]interface{}{
						"modifier": "submission_prepare",
						"addr":     value,
					},
					Err: err,
				}
			}
		}
	}

	addrs, err := mail.ParseAddressList(header.Get("From"))
	if err != nil {
		return &exterrors.SMTPError{
			Code:         554,
			EnhancedCode: exterrors.EnhancedCode{5, 6, 0},
			Message:      fmt.Sprintf("Invalid address in From"),
			Misc: map[string]interface{}{
				"modifier": "submission_prepare",
				"addr":     header.Get("From"),
			},
			Err: err,
		}
	}

	// https://tools.ietf.org/html/rfc5322#section-3.6.2
	// If From contains multiple addresses, Sender field must be present.
	if len(addrs) > 1 && header.Get("Sender") == "" {
		return &exterrors.SMTPError{
			Code:         554,
			EnhancedCode: exterrors.EnhancedCode{5, 6, 0},
			Message:      "Missing Sender header field",
			Misc: map[string]interface{}{
				"modifier": "submission_prepare",
				"from":     header.Get("From"),
			},
		}
	}

	if dateHdr := header.Get("Date"); dateHdr != "" {
		_, err := parseMessageDateTime(dateHdr)
		if err != nil {
			return &exterrors.SMTPError{
				Code:    554,
				Message: "Malformed Date header",
				Misc: map[string]interface{}{
					"modifier": "submission_prepare",
					"date":     dateHdr,
				},
				Err: err,
			}
		}
	} else {
		s.log.Msg("adding missing Date header")
		header.Set("Date", now().UTC().Format("Mon, 2 Jan 2006 15:04:05 -0700"))
	}

	return nil
}
