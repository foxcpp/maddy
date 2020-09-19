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

package imapsql

import (
	"fmt"
	"strings"

	"github.com/foxcpp/maddy/framework/address"
	"github.com/foxcpp/maddy/framework/dns"
	"golang.org/x/text/secure/precis"
)

var normalizeFuncs = map[string]func(string) (string, error){
	"precis_email": func(s string) (string, error) {
		mbox, domain, err := address.Split(s)
		if err != nil {
			return "", fmt.Errorf("imapsql: username prepare: %w", err)
		}

		// PRECIS is not included in the regular address.ForLookup since it reduces
		// the range of valid addresses to a subset of actually valid values.
		// PRECIS is a matter of our own local policy, not a general rule for all
		// email addresses.

		// Side note: For used profiles, there is no practical difference between
		// CompareKey and String.
		mbox, err = precis.UsernameCaseMapped.CompareKey(mbox)
		if err != nil {
			return "", fmt.Errorf("imapsql: username prepare: %w", err)
		}

		domain, err = dns.ForLookup(domain)
		if err != nil {
			return "", fmt.Errorf("imapsql: username prepare: %w", err)
		}

		return mbox + "@" + domain, nil
	},
	"precis": func(s string) (string, error) {
		return precis.UsernameCaseMapped.CompareKey(s)
	},
	"casefold": func(s string) (string, error) {
		return strings.ToLower(s), nil
	},
	"no": func(s string) (string, error) {
		return s, nil
	},
}
