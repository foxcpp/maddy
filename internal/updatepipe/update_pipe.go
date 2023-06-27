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

// Package updatepipe implements utilities for serialization and transport of
// IMAP update objects between processes and machines.
//
// Its main goal is provide maddy command with ability to properly notify the
// server about changes without relying on it to coordinate access in the
// first place (so maddy command can work without a running server or with a
// broken server instance).
//
// Additionally, it can be used to transfer IMAP updates between replicated
// nodes.
package updatepipe

import (
	mess "github.com/foxcpp/go-imap-mess"
)

// The P interface represents the handle for a transport medium used for IMAP
// updates.
type P interface {
	// Listen starts the "pull" goroutine that reads updates from the pipe and
	// sends them to the channel.
	//
	// Usually it is not possible to call Listen multiple times for the same
	// pipe.
	//
	// Updates sent using the same UpdatePipe object using Push are not
	// duplicates to the channel passed to Listen.
	Listen(upds chan<- mess.Update) error

	// InitPush prepares the UpdatePipe to be used as updates source (Push
	// method).
	//
	// It is called implicitly on the first Push call, but calling it
	// explicitly allows to detect initialization errors early.
	InitPush() error

	// Push writes the update to the pipe.
	//
	// The update will not be duplicated if the UpdatePipe is also listening
	// for updates.
	Push(upd mess.Update) error

	Close() error
}
