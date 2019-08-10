package module

import (
	"github.com/emersion/go-message/textproto"
)

type DeliveryTarget interface {
	Start(ctx *DeliveryContext, mailFrom string) (Delivery, error)
}

type Delivery interface {
	AddRcpt(rcptTo string) error
	Body(header textproto.Header, body BodyBuffer) error

	Abort() error
	Commit() error
}
