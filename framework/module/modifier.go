package module

import (
	"context"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/framework/buffer"
)

// Modifier is the module interface for modules that can mutate the
// processed message or its meta-data.
//
// Currently, the message body can't be mutated for efficiency and
// correctness reasons: It would require "rebuffering" (see buffer.Buffer doc),
// can invalidate assertions made on the body contents before modification and
// will break DKIM signatures.
//
// Only message header can be modified. Furthermore, it is highly discouraged for
// modifiers to remove or change existing fields to prevent issues outlined
// above.
//
// Calls on ModifierState are always strictly ordered.
// RewriteRcpt is newer called before RewriteSender and RewriteBody is never called
// before RewriteRcpts. This allows modificator code to save values
// passed to previous calls for use in later operations.
type Modifier interface {
	// ModStateForMsg initializes modifier "internal" state
	// required for processing of the message.
	ModStateForMsg(ctx context.Context, msgMeta *MsgMetadata) (ModifierState, error)
}

type ModifierState interface {
	// RewriteSender allows modifier to replace MAIL FROM value.
	// If no changes are required, this method returns its
	// argument, otherwise it returns a new value.
	//
	// Note that per-source/per-destination modifiers are executed
	// after routing decision is made so changed value will have no
	// effect on it.
	//
	// Also note that MsgMeta.OriginalFrom will still contain the original value
	// for purposes of tracing. It should not be modified by this method.
	RewriteSender(ctx context.Context, mailFrom string) (string, error)

	// RewriteRcpt replaces RCPT TO value.
	// If no changed are required, this method returns its argument, otherwise
	// it returns a new value.
	//
	// MsgPipeline will take of populating MsgMeta.OriginalRcpts. RewriteRcpt
	// doesn't do it.
	RewriteRcpt(ctx context.Context, rcptTo string) (string, error)

	// RewriteBody modifies passed Header argument and may optionally
	// inspect the passed body buffer to make a decision on new header field values.
	//
	// There is no way to modify the body and RewriteBody should avoid
	// removing existing header fields and changing their values.
	RewriteBody(ctx context.Context, h *textproto.Header, body buffer.Buffer) error

	// Close is called after the message processing ends, even if any of the
	// Rewrite* functions return an error.
	Close() error
}
