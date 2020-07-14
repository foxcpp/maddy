package table

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
)

type Static struct {
	modName  string
	instName string

	m map[string]string
}

func NewStatic(modName, instName string, _, _ []string) (module.Module, error) {
	return &Static{
		modName:  modName,
		instName: instName,
		m:        map[string]string{},
	}, nil
}

func (s *Static) Init(cfg *config.Map) error {
	cfg.Callback("entry", func(m *config.Map, node config.Node) error {
		if len(node.Args) != 2 {
			return config.NodeErr(node, "expected exactly two arguments")
		}
		s.m[node.Args[0]] = node.Args[1]
		return nil
	})
	_, err := cfg.Process()
	return err
}

func (s *Static) Name() string {
	return s.modName
}

func (s *Static) InstanceName() string {
	return s.modName
}

func (s *Static) Lookup(key string) (string, bool, error) {
	val, ok := s.m[key]
	return val, ok, nil
}

func init() {
	module.RegisterDeprecated("static", "table.static", NewStatic)
	module.Register("table.static", NewStatic)
}
