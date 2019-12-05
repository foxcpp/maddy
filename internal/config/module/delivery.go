package modconfig

import (
	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/module"
)

// deliveryDirective is a callback for use in config.Map.Custom.
//
// It does all work necessary to create a module instance from the config
// directive with the following structure:
// directive_name mod_name [inst_name] [{
//   inline_mod_config
// }]
//
// Note that if used configuration structure lacks directive_name before mod_name - this function
// should not be used (call DeliveryTarget directly).
func DeliveryDirective(m *config.Map, node *config.Node) (interface{}, error) {
	return DeliveryTarget(m.Globals, node.Args, node)
}

func DeliveryTarget(globals map[string]interface{}, args []string, block *config.Node) (module.DeliveryTarget, error) {
	var target module.DeliveryTarget
	if err := ModuleFromNode(args, block, globals, &target); err != nil {
		return nil, err
	}
	return target, nil
}
