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

package config

import (
	"reflect"
	"testing"
)

func TestStandardizeAddress(t *testing.T) {
	for _, expected := range []Endpoint{
		{Original: "tcp://0.0.0.0:10025", Scheme: "tcp", Host: "0.0.0.0", Port: "10025"},
		{Original: "tcp://[::]:10025", Scheme: "tcp", Host: "::", Port: "10025"},
		{Original: "tcp:127.0.0.1:10025", Scheme: "tcp", Host: "127.0.0.1", Port: "10025"},
		{Original: "unix://path", Scheme: "unix", Host: "", Path: "path", Port: ""},
		{Original: "unix:path", Scheme: "unix", Host: "", Path: "path", Port: ""},
		{Original: "unix:/path", Scheme: "unix", Host: "", Path: "/path", Port: ""},
		{Original: "unix:///path", Scheme: "unix", Host: "", Path: "/path", Port: ""},
		{Original: "unix://also/path", Scheme: "unix", Host: "", Path: "also/path", Port: ""},
		{Original: "unix:///also/path", Scheme: "unix", Host: "", Path: "/also/path", Port: ""},
		{Original: "tls://0.0.0.0:10025", Scheme: "tls", Host: "0.0.0.0", Port: "10025"},
		{Original: "tls:0.0.0.0:10025", Scheme: "tls", Host: "0.0.0.0", Port: "10025"},
	} {
		actual, err := ParseEndpoint(expected.Original)
		if err != nil {
			t.Errorf("Unexpected failure for %s: %v", expected.Original, err)
			return
		}

		if !reflect.DeepEqual(expected, actual) {
			t.Errorf("Didn't parse URL %q correctly\ngot %#v\nwant %#v", expected.Original, actual, expected)
			continue
		}

		if actual.String() != expected.Original {
			t.Errorf("actual.String() = %s, want %s", actual.String(), expected.Original)
		}
	}
}
