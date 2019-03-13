package module

import (
	"bytes"
	"errors"
	"net"
)

// DeliveryContext structure is created for each message and passed with it
// to each step of pipeline.
//
// Module instances must not retain references to context structure after
// Apply function returns.
type DeliveryContext struct {
	// Message sender address.
	From string

	// Recipient addresses as specified by message sender.
	// Pipeline steps are allowed to modify this field.
	To []string

	// If message is received over SMTP and client is authenticated
	// non-anonymously - this field contains used username.
	AuthUser string

	// If message is received over SMTP - this field contains
	// FQDN sent by client in EHLO/HELO command.
	SrcHostname string
	// If message is received over SMTP - this field contains
	// network address of client.
	SrcIP net.Addr

	// Arbitrary context meta-data that can be modified by any module in
	// pipeline. It is passed unchanged to next module in chain.
	//
	// For example, spam filter may set Ctx[spam] to true to tell storage
	// backend to mark message as spam.
	Ctx map[string]interface{}

	// Custom options passed to filter from server configuration.
	Opts map[string]interface{}
}

var ErrSilentDrop = errors.New("message is dropped by filter")

// Filter is the interface implemented by modules that can perform arbitrary
// transformations on incoming/outdoing messages.
type Filter interface {
	// Apply is called for each message that should be processed by module
	// instance.
	//
	// If it returns non-nil error - message processing stops and error will be
	// returned to message source.  However, if Apply returns ErrSilentDrop
	// error value - message processing stops and message source will be told
	// that message is processed successfully.
	Apply(ctx *DeliveryContext, rcpt *bytes.Buffer) error
}

// DeliveryTarget is a special versionof Filter tht can't mutate message or its
// delivery context.
type DeliveryTarget interface {
	// Deliver is called for each message that should be processed by module
	// instance.
	//
	// If it returns non-nil error - message processing stops and error will be
	// returned to message source.
	Deliver(ctx DeliveryContext, rcpt bytes.Buffer) error
}
