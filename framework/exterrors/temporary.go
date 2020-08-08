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

package exterrors

import (
	"errors"
)

type TemporaryErr interface {
	Temporary() bool
}

// IsTemporaryOrUnspec is similar to IsTemporary except that it returns true
// if error does not have a Temporary() method. Basically, it assumes that
// errors are temporary by default compared to IsTemporary that assumes
// errors are permanent by default.
func IsTemporaryOrUnspec(err error) bool {
	var temp TemporaryErr
	if errors.As(err, &temp) {
		return temp.Temporary()
	}
	return true
}

// IsTemporary returns true whether the passed error object
// have a Temporary() method and it returns true.
func IsTemporary(err error) bool {
	var temp TemporaryErr
	if errors.As(err, &temp) {
		return temp.Temporary()
	}
	return false
}

type temporaryErr struct {
	err  error
	temp bool
}

func (t temporaryErr) Unwrap() error {
	return t.err
}

func (t temporaryErr) Error() string {
	return t.err.Error()
}

func (t temporaryErr) Temporary() bool {
	return t.temp
}

// WithTemporary wraps the passed error object with the implementation of the
// Temporary() method that will return the specified value.
//
// Original error value can be obtained using errors.Unwrap.
func WithTemporary(err error, temporary bool) error {
	return temporaryErr{err, temporary}
}
