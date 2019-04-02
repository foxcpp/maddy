package config

import (
	"os"
	"strings"
)

func expandEnvironment(nodes []Node) ([]Node, error) {
	replacer := buildEnvReplacer()
	newNodes := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		node.Name = replacer.Replace(node.Name)
		for i, arg := range node.Args {
			node.Args[i] = replacer.Replace(arg)
		}
		var err error
		node.Children, err = expandEnvironment(node.Children)
		if err != nil {
			return nil, err
		}
		newNodes = append(newNodes, node)
	}
	return newNodes, nil
}

func buildEnvReplacer() *strings.Replacer {
	env := os.Environ()
	pairs := make([]string, 0, len(env)*4)
	for _, entry := range env {
		parts := strings.SplitN(entry, "=", 2)
		key := parts[0]
		value := parts[1]

		pairs = append(pairs, "{%"+key+"%}", value)
		pairs = append(pairs, "{$"+key+"}", value)
	}
	return strings.NewReplacer(pairs...)
}
