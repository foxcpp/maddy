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

func MsgModifier(globals map[string]interface{}, args []string, block *config.Node) (module.Modifier, error) {
	var check module.Modifier
	if err := ModuleFromNode(args, block, globals, &check); err != nil {
		return nil, err
	}
	return check, nil
}

func StorageDirective(m *config.Map, node *config.Node) (interface{}, error) {
	var backend module.Storage
	if err := ModuleFromNode(node.Args, node, m.Globals, &backend); err != nil {
		return nil, err
	}
	return backend, nil
}

func TableDirective(m *config.Map, node *config.Node) (interface{}, error) {
	var tbl module.Table
	if err := ModuleFromNode(node.Args, node, m.Globals, &tbl); err != nil {
		return nil, err
	}
	return tbl, nil
}
