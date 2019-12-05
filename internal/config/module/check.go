package modconfig

import (
	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/module"
)

func MessageCheck(globals map[string]interface{}, args []string, block *config.Node) (module.Check, error) {
	var check module.Check
	if err := ModuleFromNode(args, block, globals, &check); err != nil {
		return nil, err
	}
	return check, nil
}
