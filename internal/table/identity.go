package table

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
)

type Identity struct {
	modName  string
	instName string
}

func NewIdentity(modName, instName string, _, _ []string) (module.Module, error) {
	return &Identity{
		modName:  modName,
		instName: instName,
	}, nil
}

func (s *Identity) Init(cfg *config.Map) error {
	return nil
}

func (s *Identity) Name() string {
	return s.modName
}

func (s *Identity) InstanceName() string {
	return s.modName
}

func (s *Identity) Lookup(key string) (string, bool, error) {
	return key, true, nil
}

func init() {
	module.RegisterDeprecated("identity", "table.identity", NewIdentity)
	module.Register("table.identity", NewIdentity)
}
