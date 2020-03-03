package main

import (
	"errors"
	"os"

	"github.com/foxcpp/maddy"
	"github.com/foxcpp/maddy/internal/config"
	parser "github.com/foxcpp/maddy/pkg/cfgparser"
)

func findBlockInCfg(path, cfgBlock string) (root, block config.Node, err error) {
	f, err := os.Open(path)
	nodes, err := parser.Read(f, path)
	if err != nil {
		return config.Node{}, config.Node{}, err
	}
	defer f.Close()

	globals := config.NewMap(nil, config.Node{Children: nodes})
	globals.String("state_dir", false, false, maddy.DefaultStateDirectory, &config.StateDirectory)
	globals.String("runtime_dir", false, false, maddy.DefaultRuntimeDirectory, &config.RuntimeDirectory)

	// We don't care about other directives, but permit them.
	globals.AllowUnknown()

	if _, err := globals.Process(); err != nil {
		return config.Node{}, config.Node{}, err
	}

	if err := maddy.InitDirs(); err != nil {
		return config.Node{}, config.Node{}, err
	}

	for _, node := range nodes {
		if len(node.Args) == 0 && cfgBlock == node.Name {
			return config.Node{Children: nodes}, node, nil
		}

		for _, arg := range node.Args {
			if arg == cfgBlock {
				return config.Node{Children: nodes}, node, nil
			}
		}
	}

	return config.Node{}, config.Node{}, errors.New("no requested block found in configuration")
}
