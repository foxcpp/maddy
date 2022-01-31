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
	IMAPFilter(accountName string, rcptTo string, meta *MsgMetadata, hdr textproto.Header, body buffer.Buffer) (folder string, flags []string, err error)
}
