package sql

import "github.com/emersion/go-imap/backend"

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

	// TODO: PRECIS OpaqueString for password.
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

	return store.Back.SetUserPassword(accountName, newPassword)
}

func (store *Storage) GetUser(username string) (backend.User, error) {
	accountName, err := prepareUsername(username)
	if err != nil {
		return nil, err
	}

	return store.Back.GetUser(accountName)
}
