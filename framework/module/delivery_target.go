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
	"github.com/foxcpp/maddy/framework/buffer"
)

// DeliveryTarget interface represents abstract storage for the message data
// (typically persistent) or other kind of component that can be used as a
// final destination for the message.
//
// Modules implementing this interface should be registered with "target."
// prefix in name.
type DeliveryTarget interface {
	// Start starts the delivery of a new message.
	//
	// The domain part of the MAIL FROM address is assumed to be U-labels with
	// NFC normalization and case-folding applied. The message source should
	// ensure that by calling address.CleanDomain if necessary.
	Start(ctx context.Context, msgMeta *MsgMetadata, mailFrom string) (Delivery, error)
}

type Delivery interface {
	// AddRcpt adds the target address for the message.
	//
	// The domain part of the address is assumed to be U-labels with NFC normalization
	// and case-folding applied. The message source should ensure that by
	// calling address.CleanDomain if necessary.
	//
	// Implementation should assume that no case-folding or deduplication was
	// done by caller code. Its implementation responsibility to do so if it is
	// necessary. It is not recommended to reject duplicated recipients,
	// however. They should be silently ignored.
	//
	// Implementation should do as much checks as possible here and reject
	// recipients that can't be used.  Note: MsgMetadata object passed to Start
	// contains BodyLength field. If it is non-zero, it can be used to check
	// storage quota for the user before Body.
	AddRcpt(ctx context.Context, rcptTo string) error

	// Body sets the body and header contents for the message.
	// If this method fails, message is assumed to be undeliverable
	// to all recipients.
	//
	// Implementation should avoid doing any persistent changes to the
	// underlying storage until Commit is called. If that is not possible,
	// Abort should (attempt to) rollback any such changes.
	//
	// If Body can't be implemented without per-recipient failures,
	// then delivery object should also implement PartialDelivery interface
	// for use by message sources that are able to make sense of per-recipient
	// errors.
	//
	// Here is the example of possible implementation for maildir-based
	// storage:
	// Calling Body creates a file in tmp/ directory.
	// Commit moves the created file to new/ directory.
	// Abort removes the created file.
	Body(ctx context.Context, header textproto.Header, body buffer.Buffer) error

	// Abort cancels message delivery.
	//
	// All changes made to the underlying storage should be aborted at this
	// point, if possible.
	Abort(ctx context.Context) error

	// Commit completes message delivery.
	//
	// It generally should never fail, since failures here jeopardize
	// atomicity of the delivery if multiple targets are used.
	Commit(ctx context.Context) error
}
