//+build !nosqlite3,cgo

package table

import (
	"path/filepath"
	"testing"

	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/testutils"
)

func TestSQL(t *testing.T) {
	path := testutils.Dir(t)
	mod, err := NewSQL("sql_table", "", nil, nil)
	if err != nil {
		t.Fatal("Module create failed:", err)
	}
	tbl := mod.(*SQL)
	err = tbl.Init(config.NewMap(nil, &config.Node{
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
					"CREATE TABLE testTbl (key TEXT PRIMARY KEY , value TEXT)",
					"INSERT INTO testTbl VALUES ('user1', 'user1a')",
					"INSERT INTO testTbl VALUES ('user3', NULL)",
				},
			},
			{
				Name: "lookup",
				Args: []string{"SELECT value FROM testTbl WHERE key = $1"},
			},
		},
	}))
	if err != nil {
		t.Fatal("Init failed:", err)
	}

	check := func(key, res string, ok, fail bool) {
		t.Helper()

		actualRes, actualOk, err := tbl.Lookup(key)
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
}
