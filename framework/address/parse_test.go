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

import "testing"

func TestParseAddressListEncodedDisplayNames(t *testing.T) {
	tests := []struct {
		name string
		list string
	}{
		{
			name: "KOI8-R",
			list: `=?koi8-r?B?9MXT1A==?= <test@example.org>`,
		},
		{
			name: "Windows-1251",
			list: `=?windows-1251?B?0uXx8g==?= <test@example.org>`,
		},
		{
			name: "UTF8 label",
			list: `=?utf8?B?0KLQtdGB0YI=?= <test@example.org>`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			list, err := ParseList(tc.list)
			if err != nil {
				t.Fatalf("ParseAddressList() error = %v", err)
			}
			if len(list) != 1 {
				t.Fatalf("ParseAddressList() returned %d addresses, want 1", len(list))
			}
			if list[0].Address != "test@example.org" {
				t.Errorf("address = %q, want %q", list[0].Address, "test@example.org")
			}
			if list[0].Name != "Тест" {
				t.Errorf("display name = %q, want %q", list[0].Name, "Тест")
			}
		})
	}
}

func TestParseAddressListErrors(t *testing.T) {
	for _, list := range []string{
		`not an address`,
		`=?unsupported-charset?Q?name?= <test@example.org>`,
	} {
		t.Run(list, func(t *testing.T) {
			if _, err := ParseList(list); err == nil {
				t.Errorf("ParseAddressList(%q) succeeded, want error", list)
			}
		})
	}
}
