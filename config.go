package maddy

import (
	"errors"
	"fmt"
	"os"

	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/log"
	"github.com/emersion/maddy/module"
)

/*
Config matchers for module interfaces.
*/

// createModule is a helper function for config matchers that can create inline modules.
func createModule(args []string) (module.Module, error) {
	modName := args[0]
	var instName string
	if len(args) >= 2 {
		instName = args[1]
		if module.HasInstance(instName) {
			return nil, fmt.Errorf("module instance named %s already exists", instName)
		}
	}

	newMod := module.Get(modName)
	if newMod == nil {
		return nil, fmt.Errorf("unknown module: %s", modName)
	}

	log.Debugln("module create", modName, instName, "(inline)")

	return newMod(modName, instName)
}

func initInlineModule(modObj module.Module, globals map[string]interface{}, node *config.Node) error {
	// This is to ensure modules Init will see expected node layout if it breaks
	// Map abstraction and works with map.Values.
	//
	// Expected: modName modArgs { ... }
	// Actual: something modName modArgs { ... }
	node.Name = node.Args[0]
	node.Args = node.Args[1:]

	log.Debugln("module init", modObj.Name(), modObj.InstanceName(), "(inline)")
	return modObj.Init(config.NewMap(globals, node))
}

func deliverDirective(m *config.Map, node *config.Node) (interface{}, error) {
	return deliverTarget(m.Globals, node)
}

func deliverTarget(globals map[string]interface{}, node *config.Node) (module.DeliveryTarget, error) {
	// First argument to make it compatible with config.Map.
	if len(node.Args) == 0 {
		return nil, config.NodeErr(node, "expected at least 1 argument")
	}

	var modObj module.Module
	var err error
	if node.Children != nil {
		modObj, err = createModule(node.Args)
		if err != nil {
			return nil, config.NodeErr(node, "%s", err.Error())
		}
	} else {
		modObj, err = module.GetInstance(node.Args[0])
		if err != nil {
			return nil, config.NodeErr(node, "%s", err.Error())
		}
	}

	target, ok := modObj.(module.DeliveryTarget)
	if !ok {
		return nil, config.NodeErr(node, "module %s doesn't implement delivery target interface", modObj.Name())
	}

	if node.Children != nil {
		if err := initInlineModule(modObj, globals, node); err != nil {
			return nil, err
		}
	}

	return target, nil
}

func authDirective(m *config.Map, node *config.Node) (interface{}, error) {
	if len(node.Args) == 0 {
		return nil, m.MatchErr("expected at least 1 argument")
	}

	var modObj module.Module
	var err error
	if node.Children != nil {
		modObj, err = createModule(node.Args)
		if err != nil {
			return nil, m.MatchErr("%s", err.Error())
		}
	} else {
		modObj, err = module.GetInstance(node.Args[0])
		if err != nil {
			return nil, m.MatchErr("%s", err.Error())
		}
	}

	provider, ok := modObj.(module.AuthProvider)
	if !ok {
		return nil, m.MatchErr("module %s doesn't implement auth. provider interface", modObj.Name())
	}

	if node.Children != nil {
		if err := initInlineModule(modObj, m.Globals, node); err != nil {
			return nil, err
		}
	}

	return provider, nil
}

func storageDirective(m *config.Map, node *config.Node) (interface{}, error) {
	if len(node.Args) == 0 {
		return nil, m.MatchErr("expected at least 1 argument")
	}

	var modObj module.Module
	var err error
	if node.Children != nil {
		modObj, err = createModule(node.Args)
		if err != nil {
			return nil, m.MatchErr("%s", err.Error())
		}
	} else {
		modObj, err = module.GetInstance(node.Args[0])
		if err != nil {
			return nil, m.MatchErr("%s", err.Error())
		}
	}

	backend, ok := modObj.(module.Storage)
	if !ok {
		return nil, m.MatchErr("module %s doesn't implement storage interface", modObj.Name())
	}

	if node.Children != nil {
		if err := initInlineModule(modObj, m.Globals, node); err != nil {
			return nil, err
		}
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
