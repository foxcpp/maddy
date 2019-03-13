package module

import imapbackend "github.com/emersion/go-imap/backend"

// Storage interface is a slightly modified go-imap's Backend interface
// (authentication is removed).
type Storage interface {
	// GetUser returns User associated with user account specified by name.
	//
	// If it doesn't exists - it should be created.
	GetUser(username string) (imapbackend.User, error)
}

// IMAPBackend is structure that binds together implementations of Storage and
// AuthProvider interfaces to get full go-imap Backend implementation.
type IMAPBackend struct {
	Auth  AuthProvider
	Store Storage
}

// Force check to ensure that we really implement go-imap interface.
var _ imapbackend.Backend = IMAPBackend{}

func (be IMAPBackend) Login(username, password string) (imapbackend.User, error) {
	if !be.Auth.CheckPlain(username, password) {
		return nil, imapbackend.ErrInvalidCredentials
	}

	return be.Store.GetUser(username)
}
