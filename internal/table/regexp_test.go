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

package table

import (
	"context"
	"testing"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/internal/testutils"
)

func TestRegexpExpandPlaceholders(t *testing.T) {
	test := func(directives []config.Node, expected string) {
		t.Helper()

		mod, err := NewRegexp(container.New(), "table.regexp", "")
		if err != nil {
			t.Fatal(err)
		}
		r := mod.(*Regexp)
		r.log = testutils.Logger(t, "table.regexp")

		cfg := &config.Map{Block: config.Node{Children: directives}}
		if err := r.Configure([]string{"(.+)@example.org", "$1@example.com"}, cfg); err != nil {
			t.Fatal(err)
		}

		val, ok, err := r.Lookup(context.Background(), "user@example.org")
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatal("no match")
		}
		if val != expected {
			t.Errorf("expected %q, got %q", expected, val)
		}
	}

	dir := func(name string, args ...string) []config.Node {
		return []config.Node{{Name: name, Args: args}}
	}

	test(nil, "user@example.com")
	test(dir("expand_placeholders"), "user@example.com")
	test(dir("expand_placeholders", "no"), "$1@example.com")
	test(dir("expand_replaceholders"), "user@example.com")
	test(dir("expand_replaceholders", "yes"), "user@example.com")
	test(dir("expand_replaceholders", "no"), "$1@example.com")
}

func TestRegexpExpandPlaceholdersInvalid(t *testing.T) {
	for _, directives := range [][]config.Node{
		{{Name: "expand_replaceholders", Args: []string{"maybe"}}},
		{{Name: "expand_replaceholders", Args: []string{"yes", "no"}}},
		{{Name: "expand_replaceholders", Children: []config.Node{{Name: "x"}}}},
		{{Name: "expand_replaceholders", Args: []string{"yes"}}, {Name: "expand_replaceholders", Args: []string{"no"}}},
	} {
		mod, err := NewRegexp(container.New(), "table.regexp", "")
		if err != nil {
			t.Fatal(err)
		}
		r := mod.(*Regexp)
		r.log = testutils.Logger(t, "table.regexp")

		cfg := &config.Map{Block: config.Node{Children: directives}}
		if err := r.Configure([]string{"(.+)@example.org", "$1@example.com"}, cfg); err == nil {
			t.Errorf("expected error for %+v", directives)
		}
	}
}
