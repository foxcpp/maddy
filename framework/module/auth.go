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

import "errors"

// ErrUnknownCredentials should be returned by auth. provider if supplied
// credentials are valid for it but are not recognized (e.g. not found in
// used DB).
var ErrUnknownCredentials = errors.New("unknown credentials")

// PlainAuth is the interface implemented by modules providing authentication using
// username:password pairs.
//
// Modules implementing this interface should be registered with "auth." prefix in name.
type PlainAuth interface {
	AuthPlain(username, password string) error
}

// PlainUserDB is a local credentials store that can be managed using maddy command
// utility.
type PlainUserDB interface {
	PlainAuth
	ListUsers() ([]string, error)
	CreateUser(username, password string) error
	SetUserPassword(username, password string) error
	DeleteUser(username string) error
}
