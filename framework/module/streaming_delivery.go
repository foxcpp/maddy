/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2025 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

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
	"io"

	"github.com/emersion/go-message/textproto"
)

// StreamingDelivery is an optional interface that may be implemented by the
// object returned by DeliveryTarget.StartDelivery to receive the message body
// while it is still being read from the source, without buffering it first.
type StreamingDelivery interface {
	// CanStreamBody reports whether BodyStream can be used instead of Body for
	// the current message. It is called after all AddRcpt calls.
	CanStreamBody() bool

	// BodyStream is similar to Body of the regular Delivery interface except
	// that the body is read exactly once, while the message source is still
	// receiving it.
	//
	// If reading body fails, BodyStream must return an error and must make
	// sure the partially read message is not delivered. The message source
	// calls Abort after a failed BodyStream.
	BodyStream(ctx context.Context, header textproto.Header, body io.Reader) error
}
