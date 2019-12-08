// Package updatepipe implements utilities for serialization and transport of
// IMAP update objects between processes and machines.
//
// Its main goal is provide maddyctl with ability to properly notify the server
// about changes without relying on it to coordinate access in the first place
// (so maddyctl can work without a running server or with a broken server
// instance).
//
// Additionally, it can be used to transfer IMAP updates between replicated
// nodes.
package updatepipe

import (
	"github.com/emersion/go-imap/backend"
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
	Listen(upds chan<- backend.Update) error

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
	Push(upd backend.Update) error

	Close() error
}
