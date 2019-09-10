// +build !cgo

package pam

import (
	"errors"
)

const canCallDirectly = false

var ErrInvalidCredentials = errors.New("pam: invalid credentials or unknown user")

func runPAMAuth(username, password string) error {
	return errors.New("pam: Can't call libpam directly")
}
