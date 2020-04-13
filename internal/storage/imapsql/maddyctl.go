package imapsql

import (
	"github.com/emersion/go-imap/backend"
)

// These methods wrap corresponding go-imap-sql methods, but also apply
// maddy-specific credentials rules.

func (store *Storage) ListIMAPAccts() ([]string, error) {
	return store.Back.ListUsers()
}

func (store *Storage) CreateIMAPAcct(username string) error {
	accountName, err := prepareUsername(username)
	if err != nil {
		return err
	}

	return store.Back.CreateUser(accountName)
}

func (store *Storage) DeleteIMAPAcct(username string) error {
	accountName, err := prepareUsername(username)
	if err != nil {
		return err
	}

	return store.Back.DeleteUser(accountName)
}

func (store *Storage) GetIMAPAcct(username string) (backend.User, error) {
	accountName, err := prepareUsername(username)
	if err != nil {
		return nil, err
	}

	return store.Back.GetUser(accountName)
}
