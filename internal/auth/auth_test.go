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

package auth

import (
	"fmt"
	"testing"
)

func TestCheckDomainAuth(t *testing.T) {
	cases := []struct {
		rawUsername string

		perDomain      bool
		allowedDomains []string

		loginName string
	}{
		{
			rawUsername: "username",
			loginName:   "username",
		},
		{
			rawUsername:    "username",
			allowedDomains: []string{"example.org"},
			loginName:      "username",
		},
		{
			rawUsername:    "username@example.org",
			allowedDomains: []string{"example.org"},
			loginName:      "username",
		},
		{
			rawUsername:    "username@example.com",
			allowedDomains: []string{"example.org"},
		},
		{
			rawUsername:    "username",
			allowedDomains: []string{"example.org"},
			perDomain:      true,
		},
		{
			rawUsername:    "username@example.com",
			allowedDomains: []string{"example.org"},
			perDomain:      true,
		},
		{
			rawUsername:    "username@EXAMPLE.Org",
			allowedDomains: []string{"exaMPle.org"},
			perDomain:      true,
			loginName:      "username@EXAMPLE.Org",
		},
		{
			rawUsername:    "username@example.org",
			allowedDomains: []string{"example.org"},
			perDomain:      true,
			loginName:      "username@example.org",
		},
	}

	for _, case_ := range cases {
		t.Run(fmt.Sprintf("%+v", case_), func(t *testing.T) {
			loginName, allowed := CheckDomainAuth(case_.rawUsername, case_.perDomain, case_.allowedDomains)
			if case_.loginName != "" && !allowed {
				t.Fatalf("Unexpected authentication fail")
			}
			if case_.loginName == "" && allowed {
				t.Fatalf("Expected authentication fail, got %s as login name", loginName)
			}

			if loginName != case_.loginName {
				t.Errorf("Incorrect login name, got %s, wanted %s", loginName, case_.loginName)
			}
		})
	}
}
