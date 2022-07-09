//go:build !nosqlite3 && cgo
// +build !nosqlite3,cgo

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
	"path/filepath"
	"reflect"
	"testing"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/internal/testutils"
)

func TestSQL(t *testing.T) {
	path := testutils.Dir(t)
	mod, err := NewSQL("sql_table", "", nil, nil)
	if err != nil {
		t.Fatal("Module create failed:", err)
	}
	tbl := mod.(*SQL)
	err = tbl.Init(config.NewMap(nil, config.Node{
		Children: []config.Node{
			{
				Name: "driver",
				Args: []string{"sqlite3"},
			},
			{
				Name: "dsn",
				Args: []string{filepath.Join(path, "test.db")},
			},
			{
				Name: "init",
				Args: []string{
					"CREATE TABLE testTbl (key TEXT, value TEXT)",
					"INSERT INTO testTbl VALUES ('user1', 'user1a')",
					"INSERT INTO testTbl VALUES ('user1', 'user1b')",
					"INSERT INTO testTbl VALUES ('user3', NULL)",
				},
			},
			{
				Name: "lookup",
				Args: []string{"SELECT value FROM testTbl WHERE key = $key"},
			},
		},
	}))
	if err != nil {
		t.Fatal("Init failed:", err)
	}

	check := func(key, res string, ok, fail bool) {
		t.Helper()

		actualRes, actualOk, err := tbl.Lookup(context.Background(), key)
		if actualRes != res {
			t.Errorf("Result mismatch: want %s, got %s", res, actualRes)
		}
		if actualOk != ok {
			t.Errorf("OK mismatch: want %v, got %v", actualOk, ok)
		}
		if (err != nil) != fail {
			t.Errorf("Error mismatch: want failure = %v, got %v", fail, err)
		}
	}

	check("user1", "user1a", true, false)
	check("user2", "", false, false)
	check("user3", "", false, true)

	vals, err := tbl.LookupMulti(context.Background(), "user1")
	if err != nil {
		t.Error("Unexpected error:", err)
	}
	if !reflect.DeepEqual(vals, []string{"user1a", "user1b"}) {
		t.Error("Wrong result of LookupMulti:", vals)
	}
}
