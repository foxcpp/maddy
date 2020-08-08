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

// shadow package implements utilities for parsing and using shadow password
// database on Unix systems.
package shadow

type Entry struct {
	// User login name.
	Name string

	// Hashed user password.
	Pass string

	// Days since Jan 1, 1970 password was last changed.
	LastChange int

	// The number of days the user will have to wait before she will be allowed to
	// change her password again.
	//
	// -1 if password aging is disabled.
	MinPassAge int

	// The number of days after which the user will have to change her password.
	//
	// -1 is password aging is disabled.
	MaxPassAge int

	// The number of days before a password is going to expire (see the maximum
	// password age above) during which the user should be warned.
	//
	// -1 is password aging is disabled.
	WarnPeriod int

	// The number of days after a password has expired (see the maximum
	// password age above) during which the password should still be accepted.
	//
	// -1 is password aging is disabled.
	InactivityPeriod int

	// The date of expiration of the account, expressed as the number of days
	// since Jan 1, 1970.
	//
	// -1 is account never expires.
	AcctExpiry int

	// Unused now.
	Flags int
}
