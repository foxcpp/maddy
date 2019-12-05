package module

import imapbackend "github.com/emersion/go-imap/backend"

// Storage interface is a slightly modified go-imap's Backend interface
// (authentication is removed).
type Storage interface {
	// GetOrCreateUser returns User associated with user account specified by
	// name.
	//
	// If it doesn't exists - it should be created.
	GetOrCreateUser(username string) (imapbackend.User, error)

	// Extensions returns list of IMAP extensions supported by backend.
	IMAPExtensions() []string
}
