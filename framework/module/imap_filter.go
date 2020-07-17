package module

import (
	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/framework/buffer"
)

// IMAPFilter is interface used by modules that want to modify IMAP-specific message
// attributes on delivery.
//
// Modules implementing this interface should be registered with namespace prefix
// "imap.filter".
type IMAPFilter interface {
	// IMAPFilter is called when message is about to be stored in IMAP-compatible
	// storage. It is called only for messages delivered over SMTP, hdr and body
	// contain the message exactly how it will be stored.
	//
	// Filter can change the target directory by returning non-empty folder value.
	// Additionally it can add additional IMAP flags to the message by returning
	// them.
	//
	// Errors returned by IMAPFilter will be just logged and will not cause delivery
	// to fail.
	IMAPFilter(accountName string, meta *MsgMetadata, hdr textproto.Header, body buffer.Buffer) (folder string, flags []string, err error)
}
