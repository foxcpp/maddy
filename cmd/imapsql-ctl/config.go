package main

import (
	"errors"
	"os"

	imapsql "github.com/foxcpp/go-imap-sql"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/config/parser"
	"github.com/foxcpp/maddy/storage/sql"
)

func backendFromCfg(path, cfgBlock string) (*imapsql.Backend, error) {
	f, err := os.Open(path)
	nodes, err := parser.Read(f, path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Global variables relevant for sql module.
	globals := config.NewMap(nil, &config.Node{Children: nodes})
	globals.Bool("auth_perdomain", false, false, nil)
	globals.StringList("auth_domains", false, false, nil, nil)
	globals.AllowUnknown()
	_, err = globals.Process()
	if err != nil {
		return nil, err
	}

	for _, node := range nodes {
		if node.Name != "sql" {
			continue
		}

		if len(node.Args) == 0 && cfgBlock == "sql" {
			return backendFromNode(globals.Values, node)
		}

		for _, arg := range node.Args {
			if arg == cfgBlock {
				return backendFromNode(globals.Values, node)
			}
		}
	}

	return nil, errors.New("no requested block found in configuration")
}

func backendFromNode(globals map[string]interface{}, node config.Node) (*imapsql.Backend, error) {
	instName := "sql"
	if len(node.Args) >= 1 {
		instName = node.Args[0]
	}

	mod, err := sql.New("sql", instName, nil, nil)
	if err != nil {
		return nil, err
	}
	if err := mod.Init(config.NewMap(globals, &node)); err != nil {
		return nil, err
	}

	return mod.(*sql.Storage).Back, nil
}
