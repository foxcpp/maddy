package msgpipeline

import (
	"github.com/foxcpp/maddy/internal/config"
	modconfig "github.com/foxcpp/maddy/internal/config/module"
	"github.com/foxcpp/maddy/internal/module"
)

// CheckGroup is a module container for a group of Check implementations.
//
// It allows to share a set of filter configurations between using named
// configuration blocks (module instances) system.
//
// It is registered globally under the name 'checks'. The object does not
// implement any standard module interfaces besides module.Module and is
// specific to the message pipeline.
type CheckGroup struct {
	instName string
	L        []module.Check
}

func (cg *CheckGroup) Init(cfg *config.Map) error {
	for _, node := range cfg.Block.Children {
		chk, err := modconfig.MessageCheck(cfg.Globals, append([]string{node.Name}, node.Args...), node)
		if err != nil {
			return err
		}

		cg.L = append(cg.L, chk)
	}

	return nil
}

func (CheckGroup) Name() string {
	return "checks"
}

func (cg CheckGroup) InstanceName() string {
	return cg.instName
}

func init() {
	module.Register("checks", func(_, instName string, _, _ []string) (module.Module, error) {
		return &CheckGroup{
			instName: instName,
		}, nil
	})
}
