package target

import (
	"net"
	"net/textproto"

	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/target/remote"
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

	if err == remote.ErrTLSRequired {
		return false
	}

	// Connection error? Assume it is temporary.
	return true
}
