package main

import (
	imapsql "github.com/foxcpp/go-imap-sql"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/storage/sql"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

func sqlFromCfgBlock(root, node *config.Node) (*imapsql.Backend, error) {
	// Global variables relevant for sql module.
	globals := config.NewMap(nil, root)
	globals.Bool("auth_perdomain", false, false, nil)
	globals.StringList("auth_domains", false, false, nil, nil)
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

	return mod.(*sql.Storage).Back, nil
}
