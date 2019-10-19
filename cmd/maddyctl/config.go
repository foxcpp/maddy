package main

import (
	"errors"
	"os"

	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/config/parser"
)

func findBlockInCfg(path, cfgBlock string) (root, block *config.Node, err error) {
	f, err := os.Open(path)
	nodes, err := parser.Read(f, path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	// Global variables relevant for sql module.
	for _, node := range nodes {
		if node.Name != "sql" {
			continue
		}

		if len(node.Args) == 0 && cfgBlock == node.Name {
			return &config.Node{Children: nodes}, &node, nil
		}

		for _, arg := range node.Args {
			if arg == cfgBlock {
				return &config.Node{Children: nodes}, &node, nil
			}
		}
	}

	return nil, nil, errors.New("no requested block found in configuration")
}
