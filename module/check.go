package module

import (
	"context"
	"sync"

	"github.com/emersion/go-message/textproto"
)

// Check is the module interface that is meant for read-only (with the
// exception of the message header modifications) (meta-)data checking.
type Check interface {
	// NewMessage initializing the "internal" check state required for
	// processing of the new message.
	NewMessage(ctx *DeliveryContext) (MessageCheck, error)
}

type MessageCheck interface {
	// CheckConnection is executed once when client sends a new message.
	//
	// Result may be cached for the whole client connection so this function
	// may not be called sometimes.
	CheckConnection(ctx context.Context) error

	// CheckSender is executed once when client sends the message sender
	// information (e.g. on the MAIL FROM command).
	CheckSender(ctx context.Context, mailFrom string) error

	// CheckRcpt is executed for each recipient when its address is received
	// from the client (e.g. on the RCPT TO command).
	CheckRcpt(ctx context.Context, rcptTo string) error

	// CheckBody is executed once after the message body is received and
	// buffered in memory or on disk.
	//
	// Check code should use passed mutex when working with the message header.
	// Body can be read without locking it since it is read-only.
	CheckBody(ctx context.Context, headerLock *sync.RWMutex, header textproto.Header, body BodyBuffer) error

	// Close is called after the message processing ends, even if any of the
	// Check* functions return an error.
	Close() error
}
