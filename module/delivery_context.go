package module

import (
	"crypto/tls"
	"net"

	"github.com/emersion/go-message/textproto"
)

// DeliveryContext structure is created for each message and passed with it
// to each step of pipeline.
//
// Module instances must not retain references to context structure after
// Apply function returns.
type DeliveryContext struct {
	// Message sender address.
	// Note that addresses may contain unescaped Unicode characters.
	From string

	// Recipient addresses as specified by message sender.
	// Pipeline steps are allowed to modify this field.
	// Note that addresses may contain unescaped Unicode characters.
	To []string

	// Set to true if message is received over SMTP and client is authenticated
	// anonymously.
	Anonymous bool

	// If message is received over SMTP and client is authenticated
	// non-anonymously - this field contains used username.
	AuthUser string
	// If message is received over SMTP and client is authenticated
	// non-anonymously - this field contains used password.
	AuthPassword string

	// IANA protocol name (SMTP, ESMTP, ESMTPS, etc)
	SrcProto string
	// If message is received over SMTPS - TLS connection state.
	SrcTLSState tls.ConnectionState
	// If message is received over SMTP - this field contains
	// FQDN sent by client in EHLO/HELO command.
	SrcHostname string
	// If message is received over SMTP - this field contains
	// network address of client.
	SrcAddr net.Addr

	// Our domain.
	OurHostname string

	Header textproto.Header

	// If set - no SrcHostname and SrcAddr will be added to Received
	// header.
	DontTraceSender bool

	// Unique identifier for this delivery attempt. Should be used in logs to
	// make troubleshooting easier.
	DeliveryID string

	// Size of message body. It is updated by pipeline code after each step
	// that changes body.
	BodyLength int

	// Arbitrary context meta-data that can be modified by any module in
	// pipeline. It is passed unchanged to next module in chain.
	//
	// For example, spam filter may set Ctx[spam] to true to tell storage
	// backend to mark message as spam.
	Ctx map[string]interface{}
}

// DeepCopy creates a copy of the DeliveryContext structure, also
// copying contents of the maps and slices.
//
// There are two exceptions, however:
// - SrcAddr is not copied and copys field references original value.
// - Slice/map/pointer values in cpy.Ctx are not copied.
func (ctx *DeliveryContext) DeepCopy() *DeliveryContext {
	cpy := *ctx
	// There is no good way to copy net.Addr, but it should not be
	// modified by anything anyway so we are safe.

	cpy.Ctx = make(map[string]interface{}, len(ctx.Ctx))
	for k, v := range ctx.Ctx {
		// TODO: This is going to cause troubes if Ctx value is itself
		// reference-based type like slice or map.
		//
		// However, since we want to have Ctx values settable and referencable
		// by configuration, they should not use any complex types.
		// Probably ints, strings, bools but not more.
		cpy.Ctx[k] = v
	}

	cpy.To = append(make([]string, 0, len(ctx.To)), ctx.To...)

	return &cpy
}
