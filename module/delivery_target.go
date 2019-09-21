package module

import (
	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/buffer"
)

// DeliveryTarget interface represents abstract storage for the message data
// (typically persistent) or other kind of component that can be used as a
// final destination for the message.
type DeliveryTarget interface {
	Start(msgMeta *MsgMetadata, mailFrom string) (Delivery, error)
}

type Delivery interface {
	// AddRcpt adds the target address for the message.
	//
	// Implementation should assume that no case-folding or
	// deduplication was done by caller code. Its implementation
	// responsibility to do so if it is necessary. It is not recommended
	// to reject duplicated recipients, however. They should be silently
	// ignored.
	//
	// Implementation should do as much checks as possible
	// here and reject recipients that can't be used.
	// Note: MsgMetadata object passed to Start contains
	// BodyLength field. If it is non-zero, it can be used
	// to check storage quota for the user before Body.
	AddRcpt(rcptTo string) error

	// Body sets the body and header contents for the message.
	// If this method fails, message is assumed to be undeliverable
	// to all recipients.
	//
	// Implementation should avoid doing any persistent changes to the
	// underlying storage until Commit is called. If that is not possible,
	// Abort should (attempt to) rollback any such changes.
	//
	// Here is the example of possible implementation for maildir-based
	// storage:
	// Calling Body creates a file in tmp/ directory.
	// Commit moves the created file to new/ directory.
	// Abort removes the created file.
	Body(header textproto.Header, body buffer.Buffer) error

	// Abort cancels message delivery.
	//
	// All changes made to the underlying storage should be aborted at this
	// point, if possible.
	Abort() error

	// Commit completes message delivery.
	//
	// if it fails or not called - no changes should be done to the storage.
	// However, sometimes (see remote module) it is not possible, see
	// queue.PartialError for the current solution to make it possible to
	// report partial delivery failures to the calling code.
	Commit() error
}
