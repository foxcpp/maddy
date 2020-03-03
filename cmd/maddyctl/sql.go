package main

import (
	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/storage/sql"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

func sqlFromCfgBlock(root, node config.Node) (*sql.Storage, error) {
	// Global variables relevant for sql module.
	globals := config.NewMap(nil, root)
	// None now...
	globals.AllowUnknown()
	_, err := globals.Process()
	if err != nil {
		return nil, err
	}

	instName := "sql"
	if len(node.Args) >= 1 {
		instName = node.Args[0]
	}

	mod, err := sql.New("sql", instName, nil, nil)
	if err != nil {
		return nil, err
	}
	if err := mod.Init(config.NewMap(globals.Values, node)); err != nil {
		return nil, err
	}

	return mod.(*sql.Storage), nil
}
