package maddy

import (
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/module"
)

/*
Config matchers for module interfaces.
*/

// createInlineModule is a helper function for config matchers that can create inline modules.
func createInlineModule(modName, instName string, aliases []string) (module.Module, error) {
	newMod := module.Get(modName)
	if newMod == nil {
		return nil, fmt.Errorf("unknown module: %s", modName)
	}

	log.Debugln("module create", modName, instName, "(inline)")

	return newMod(modName, instName, aliases)
}

// initInlineModule constructs "faked" config tree and passes it to module
// Init function to make it look like it is defined at top-level.
//
// args must contain at least one argument, otherwise initInlineModule panics.
func initInlineModule(modObj module.Module, globals map[string]interface{}, args []string, block *config.Node) error {
	log.Debugln("module init", modObj.Name(), modObj.InstanceName(), "(inline)")
	return modObj.Init(config.NewMap(globals, block))
}

// moduleFromNode does all work to create or get existing module object with a certain type.
// It is not used by top-level module definitions, only for references from other
// modules configuration blocks.
//
// inlineCfg should contain configuration directives for inline declarations.
// args should contain values that are used to create module.
// It should be either module name + instance name or just module name. Further extensions
// may add other string arguments (currently, they can be accessed by module instances
// as aliases argument to constructor).
//
// It checks using reflection whether it is possible to store a module object into modObj
// pointer (e.g. it implements all necessary interfaces) and stores it if everything is fine.
// If module object doesn't implement necessary module interfaces - error is returned.
// If modObj is not a pointer, moduleFromNode panics.
func moduleFromNode(args []string, inlineCfg *config.Node, globals map[string]interface{}, moduleIface interface{}) error {
	// single argument
	//  - instance name of an existing module
	// single argument + block
	//  - module name, inline definition
	// two arguments + block
	//  - module name and instance name, inline definition
	//
	// two arguments, no block
	//  - invalid

	if len(args) == 0 {
		return config.NodeErr(inlineCfg, "at least one argument is required")
	}

	var modObj module.Module
	var err error
	if inlineCfg.Children != nil {
		modName := args[0]

		modAliases := args[1:]
		instName := ""
		if len(args) == 2 {
			modAliases = args[2:]
			instName = args[1]
		}

		modObj, err = createInlineModule(modName, instName, modAliases)
	} else {
		if len(args) != 1 {
			return config.NodeErr(inlineCfg, "exactly one argument is required for reference to existing module")
		}
		modObj, err = module.GetInstance(args[0])
	}
	if err != nil {
		return config.NodeErr(inlineCfg, "%v", err)
	}

	// NOTE: This will panic if moduleIface is not a pointer.
	modIfaceType := reflect.TypeOf(moduleIface).Elem()
	modObjType := reflect.TypeOf(modObj)
	if !modObjType.Implements(modIfaceType) && !modObjType.AssignableTo(modIfaceType) {
		return config.NodeErr(inlineCfg, "module %s (%s) doesn't implement %v interface", modObj.Name(), modObj.InstanceName(), modIfaceType)
	}

	reflect.ValueOf(moduleIface).Elem().Set(reflect.ValueOf(modObj))

	if inlineCfg.Children != nil {
		if err := initInlineModule(modObj, globals, args, inlineCfg); err != nil {
			return err
		}
	}

	return nil
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
// should not be used (call deliveryTarget directly).
func deliveryDirective(m *config.Map, node *config.Node) (interface{}, error) {
	return deliveryTarget(m.Globals, node.Args, node)
}

func deliveryTarget(globals map[string]interface{}, args []string, block *config.Node) (module.DeliveryTarget, error) {
	var target module.DeliveryTarget
	if err := moduleFromNode(args, block, globals, &target); err != nil {
		return nil, err
	}
	return target, nil
}

func messageCheck(globals map[string]interface{}, args []string, block *config.Node) (module.Check, error) {
	var check module.Check
	if err := moduleFromNode(args, block, globals, &check); err != nil {
		return nil, err
	}
	return check, nil
}

func authDirective(m *config.Map, node *config.Node) (interface{}, error) {
	var provider module.AuthProvider
	if err := moduleFromNode(node.Args, node, m.Globals, &provider); err != nil {
		return nil, err
	}
	return provider, nil
}

func storageDirective(m *config.Map, node *config.Node) (interface{}, error) {
	var backend module.Storage
	if err := moduleFromNode(node.Args, node, m.Globals, &backend); err != nil {
		return nil, err
	}
	return backend, nil
}

func logOutput(m *config.Map, node *config.Node) (interface{}, error) {
	if len(node.Args) == 0 {
		return nil, m.MatchErr("expected at least 1 argument")
	}
	if len(node.Children) != 0 {
		return nil, m.MatchErr("can't declare block here")
	}

	outs := make([]log.FuncLog, 0, len(node.Args))
	for _, arg := range node.Args {
		switch arg {
		case "stderr":
			outs = append(outs, log.StderrLog())
		case "syslog":
			syslogOut, err := log.Syslog()
			if err != nil {
				return nil, fmt.Errorf("failed to connect to syslog daemon: %v", err)
			}
			outs = append(outs, syslogOut)
		case "off":
			if len(node.Args) != 1 {
				return nil, errors.New("'off' can't be combined with other log targets")
			}
			return nil, nil
		default:
			w, err := os.OpenFile(arg, os.O_RDWR, os.ModePerm)
			if err != nil {
				return nil, fmt.Errorf("failed to create log file: %v", err)
			}

			outs = append(outs, log.WriterLog(w))
		}
	}

	if len(outs) == 1 {
		return outs[0], nil
	}
	return log.MultiLog(outs...), nil
}

func defaultLogOutput() (interface{}, error) {
	return log.StderrLog(), nil
}
