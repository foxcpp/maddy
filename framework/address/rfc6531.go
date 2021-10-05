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

package address

import (
	"errors"

	"golang.org/x/net/idna"
	"golang.org/x/text/unicode/norm"
)

var ErrUnicodeMailbox = errors.New("address: cannot convert the Unicode local-part to the ACE form")

// ToASCII converts the domain part of the email address to the A-label form and
// fails with ErrUnicodeMailbox if the local-part contains non-ASCII characters.
func ToASCII(addr string) (string, error) {
	mbox, domain, err := Split(addr)
	if err != nil {
		return addr, err
	}

	for _, ch := range mbox {
		if ch > 128 {
			return addr, ErrUnicodeMailbox
		}
	}

	if domain == "" {
		return mbox, nil
	}

	aDomain, err := idna.ToASCII(domain)
	if err != nil {
		return addr, err
	}

	return mbox + "@" + aDomain, nil
}

// ToUnicode converts the domain part of the email address to the U-label form.
func ToUnicode(addr string) (string, error) {
	mbox, domain, err := Split(addr)
	if err != nil {
		return norm.NFC.String(addr), err
	}

	if domain == "" {
		return mbox, nil
	}

	uDomain, err := idna.ToUnicode(domain)
	if err != nil {
		return norm.NFC.String(addr), err
	}

	return mbox + "@" + norm.NFC.String(uDomain), nil
}

// SelectIDNA is a convenience function for conversion of domains in the email
// addresses to/from the Punycode form.
//
// ulabel=true => ToUnicode is used.
// ulabel=false => ToASCII is used.
func SelectIDNA(ulabel bool, addr string) (string, error) {
	if ulabel {
		return ToUnicode(addr)
	}
	return ToASCII(addr)
}
