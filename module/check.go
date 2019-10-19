package module

import (
	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-msgauth/authres"
	"github.com/foxcpp/maddy/buffer"
)

// Check is the module interface that is meant for read-only (with the
// exception of the message header modifications) (meta-)data checking.
type Check interface {
	// CheckStateForMsg initializes the "internal" check state required for
	// processing of the new message.
	CheckStateForMsg(msgMeta *MsgMetadata) (CheckState, error)
}

type CheckState interface {
	// CheckConnection is executed once when client sends a new message.
	//
	// Result may be cached for the whole client connection so this function
	// may not be called sometimes.
	CheckConnection() CheckResult

	// CheckSender is executed once when client sends the message sender
	// information (e.g. on the MAIL FROM command).
	CheckSender(mailFrom string) CheckResult

	// CheckRcpt is executed for each recipient when its address is received
	// from the client (e.g. on the RCPT TO command).
	CheckRcpt(rcptTo string) CheckResult

	// CheckBody is executed once after the message body is received and
	// buffered in memory or on disk.
	//
	// Check code should use passed mutex when working with the message header.
	// Body can be read without locking it since it is read-only.
	CheckBody(header textproto.Header, body buffer.Buffer) CheckResult

	// Close is called after the message processing ends, even if any of the
	// Check* functions return an error.
	Close() error
}

type CheckResult struct {
	// RejectErr is the error that is reported to the message source
	// if check decided that the message should be rejected.
	RejectErr error

	// Quarantine is the flag that specifies that the message
	// is considered "possibly malicious" and should be
	// put into Junk mailbox.
	//
	// This value is copied into MsgMetadata by the msgpipeline.
	Quarantine bool

	// AuthResult is the information that is supposed to
	// be included in Authentication-Results header.
	AuthResult []authres.Result

	// Header is the header fields that should be
	// added to the header after all checks.
	Header textproto.Header
}
