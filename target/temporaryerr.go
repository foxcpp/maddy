package target

import (
	"errors"
	"net"
	"net/textproto"

	"github.com/emersion/go-smtp"
)

func IsTemporaryErr(err error) bool {
	if protoErr, ok := err.(*textproto.Error); ok {
		return (protoErr.Code / 100) == 4
	}
	if smtpErr, ok := err.(*smtp.SMTPError); ok {
		return (smtpErr.Code / 100) == 4
	}
	if netErr, ok := err.(net.Error); ok {
		return netErr.Temporary()
	}

	if err == ErrTLSRequired {
		return false
	}

	// Connection error? Assume it is temporary.
	return true
}

var ErrTLSRequired = errors.New("TLS is required for outgoing connections but target server doesn't support STARTTLS")
