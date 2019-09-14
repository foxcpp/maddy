package modconfig

import (
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/module"
)

func MsgModifier(globals map[string]interface{}, args []string, block *config.Node) (module.Modifier, error) {
	var check module.Modifier
	if err := ModuleFromNode(args, block, globals, &check); err != nil {
		return nil, err
	}
	return check, nil
}
