package modconfig

import (
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/module"
)

func AuthDirective(m *config.Map, node *config.Node) (interface{}, error) {
	var provider module.AuthProvider
	if err := ModuleFromNode(node.Args, node, m.Globals, &provider); err != nil {
		return nil, err
	}
	return provider, nil
}
