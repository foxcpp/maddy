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

import "context"

// Table is the interface implemented by module that implementation string-to-string
// translation.
//
// Modules implementing this interface should be registered with prefix
// "table." in name.
type Table interface {
	Lookup(ctx context.Context, s string) (string, bool, error)
}

// MultiTable is the interface that module can implement in addition to Table
// if it can provide multiple values as a lookup result.
type MultiTable interface {
	LookupMulti(ctx context.Context, s string) ([]string, error)
}

type MutableTable interface {
	Table
	Keys() ([]string, error)
	RemoveKey(k string) error
	SetKey(k, v string) error
}
