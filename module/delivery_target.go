package module

import (
	"github.com/emersion/go-message/textproto"
	"github.com/emersion/maddy/buffer"
)

type DeliveryTarget interface {
	Start(msgMeta *MsgMetadata, mailFrom string) (Delivery, error)
}

type Delivery interface {
	AddRcpt(rcptTo string) error
	Body(header textproto.Header, body buffer.Buffer) error

	Abort() error
	Commit() error
}
