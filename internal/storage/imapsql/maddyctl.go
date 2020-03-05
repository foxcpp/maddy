package imapsql

import (
	"errors"

	"github.com/emersion/go-imap/backend"
	"golang.org/x/text/secure/precis"
)

// These methods wrap corresponding go-imap-sql methods, but also apply
// maddy-specific credentials rules.

func (store *Storage) ListUsers() ([]string, error) {
	return store.Back.ListUsers()
}

func (store *Storage) CreateUser(username, password string) error {
	accountName, err := prepareUsername(username)
	if err != nil {
		return err
	}

	password, err = precis.OpaqueString.CompareKey(password)
	if err != nil {
		return err
	}

	if len(password) == 0 {
		return errors.New("sql: empty passwords are not allowed")
	}

	return store.Back.CreateUser(accountName, password)
}

func (store *Storage) CreateUserNoPass(username string) error {
	accountName, err := prepareUsername(username)
	if err != nil {
		return err
	}

	return store.Back.CreateUserNoPass(accountName)
}

func (store *Storage) DeleteUser(username string) error {
	accountName, err := prepareUsername(username)
	if err != nil {
		return err
	}

	return store.Back.DeleteUser(accountName)
}

func (store *Storage) SetUserPassword(username, newPassword string) error {
	accountName, err := prepareUsername(username)
	if err != nil {
		return err
	}

	newPassword, err = precis.OpaqueString.CompareKey(newPassword)
	if err != nil {
		return err
	}

	if len(newPassword) == 0 {
		return errors.New("sql: empty passwords are not allowed")
	}

	return store.Back.SetUserPassword(accountName, newPassword)
}

func (store *Storage) GetUser(username string) (backend.User, error) {
	accountName, err := prepareUsername(username)
	if err != nil {
		return nil, err
	}

	return store.Back.GetUser(accountName)
}
