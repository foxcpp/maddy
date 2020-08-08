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
	imapbackend "github.com/emersion/go-imap/backend"
)

// Storage interface is a slightly modified go-imap's Backend interface
// (authentication is removed).
//
// Modules implementing this interface should be registered with prefix
// "storage." in name.
type Storage interface {
	// GetOrCreateIMAPAcct returns User associated with storage account specified by
	// the name.
	//
	// If it doesn't exists - it should be created.
	GetOrCreateIMAPAcct(username string) (imapbackend.User, error)
	GetIMAPAcct(username string) (imapbackend.User, error)

	// Extensions returns list of IMAP extensions supported by backend.
	IMAPExtensions() []string
}

// ManageableStorage is an extended Storage interface that allows to
// list existing accounts, create and delete them.
type ManageableStorage interface {
	Storage

	ListIMAPAccts() ([]string, error)
	CreateIMAPAcct(username string) error
	DeleteIMAPAcct(username string) error
}
