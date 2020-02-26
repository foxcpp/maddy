package modconfig

import (
	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/module"
)

func AuthDirective(m *config.Map, node *config.Node) (interface{}, error) {
	var provider module.PlainAuth
	if err := ModuleFromNode(node.Args, node, m.Globals, &provider); err != nil {
		return nil, err
	}
	return provider, nil
}
