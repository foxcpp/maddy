/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package module

import (
	"context"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-msgauth/authres"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/framework/buffer"
)

// Check is the module interface that is meant for read-only (with the
// exception of the message header modifications) (meta-)data checking.
//
// Modules implementing this interface should be registered with "check."
// prefix in name.
type Check interface {
	// CheckStateForMsg initializes the "internal" check state required for
	// processing of the new message.
	//
	// NOTE: Returned CheckState object must be hashable (usable as a map key).
	// This is used to deduplicate Check* calls, the easiest way to achieve
	// this is to have CheckState as a pointer to some struct, all pointers
	// are hashable.
	CheckStateForMsg(ctx context.Context, msgMeta *MsgMetadata) (CheckState, error)
}

// EarlyCheck is an optional module interface that can be implemented
// by module implementing Check.
//
// It is used as an optimization to reject obviously malicious connections
// before allocating resources for SMTP session.
//
// The Status of this check is accept (no error) or reject (error) only, no
// advanced handling is available (such as 'quarantine' action and headers
// prepending).
type EarlyCheck interface {
	CheckConnection(ctx context.Context, state *smtp.ConnectionState) error
}

type CheckState interface {
	// CheckConnection is executed once when client sends a new message.
	//
	// Result may be cached for the whole client connection so this function
	// may not be called sometimes.
	CheckConnection(ctx context.Context) CheckResult

	// CheckSender is executed once when client sends the message sender
	// information (e.g. on the MAIL FROM command).
	CheckSender(ctx context.Context, mailFrom string) CheckResult

	// CheckRcpt is executed for each recipient when its address is received
	// from the client (e.g. on the RCPT TO command).
	CheckRcpt(ctx context.Context, rcptTo string) CheckResult

	// CheckBody is executed once after the message body is received and
	// buffered in memory or on disk.
	//
	// Check code should use passed mutex when working with the message header.
	// Body can be read without locking it since it is read-only.
	CheckBody(ctx context.Context, header textproto.Header, body buffer.Buffer) CheckResult

	// Close is called after the message processing ends, even if any of the
	// Check* functions return an error.
	Close() error
}

type CheckResult struct {
	// Reason is the error that is reported to the message source
	// if check decided that the message should be rejected.
	Reason error

	// Reject is the flag that specifies that the message
	// should be rejected.
	Reject bool

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
