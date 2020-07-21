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

package dns

import (
	"golang.org/x/net/idna"
	"golang.org/x/text/unicode/norm"
)

// SelectIDNA is a convenience function for encoding to/from Punycode.
//
// If ulabel is true, it returns U-label encoded domain in the Unicode NFC
// form.
// If ulabel is false, it returns A-label encoded domain.
func SelectIDNA(ulabel bool, domain string) (string, error) {
	if ulabel {
		uDomain, err := idna.ToUnicode(domain)
		return norm.NFC.String(uDomain), err
	}
	return idna.ToASCII(domain)
}
