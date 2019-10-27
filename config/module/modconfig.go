// Package modconfig provides matchers for config.Map that query
// modules registry and parse inline module definitions.
//
// They should be used instead of manual querying when there is need to
// reference a module instance in the configuration.
//
// See ModuleFromNode documentation for explanation of what is 'args'
// for some functions (DeliveryTarget).
package modconfig

import (
	"fmt"
	"reflect"

	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/config/parser"
	"github.com/foxcpp/maddy/module"
)

// createInlineModule is a helper function for config matchers that can create inline modules.
func createInlineModule(modName string, args []string) (module.Module, error) {
	newMod := module.Get(modName)
	if newMod == nil {
		return nil, fmt.Errorf("unknown module: %s", modName)
	}

	return newMod(modName, "", nil, args)
}

// initInlineModule constructs "faked" config tree and passes it to module
// Init function to make it look like it is defined at top-level.
//
// args must contain at least one argument, otherwise initInlineModule panics.
func initInlineModule(modObj module.Module, globals map[string]interface{}, block *config.Node) error {
	return modObj.Init(config.NewMap(globals, block))
}

// ModuleFromNode does all work to create or get existing module object with a certain type.
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
// If modObj is not a pointer, ModuleFromNode panics.
func ModuleFromNode(args []string, inlineCfg *config.Node, globals map[string]interface{}, moduleIface interface{}) error {
	// single argument
	//  - instance name of an existing module
	// single argument + block
	//  - module name, inline definition
	// two+ arguments + block
	//  - module name and instance name, inline definition
	// two+ arguments, no block
	//  - module name and instance name, inline definition, empty config block

	if len(args) == 0 {
		return parser.NodeErr(inlineCfg, "at least one argument is required")
	}

	var modObj module.Module
	var err error
	if inlineCfg.Children != nil || len(args) > 1 {
		modObj, err = createInlineModule(args[0], args[1:])
	} else {
		if len(args) != 1 {
			return parser.NodeErr(inlineCfg, "exactly one argument is to use existing config block")
		}
		modObj, err = module.GetInstance(args[0])
	}
	if err != nil {
		return err
	}

	// NOTE: This will panic if moduleIface is not a pointer.
	modIfaceType := reflect.TypeOf(moduleIface).Elem()
	modObjType := reflect.TypeOf(modObj)
	if !modObjType.Implements(modIfaceType) && !modObjType.AssignableTo(modIfaceType) {
		return parser.NodeErr(inlineCfg, "module %s (%s) doesn't implement %v interface", modObj.Name(), modObj.InstanceName(), modIfaceType)
	}

	reflect.ValueOf(moduleIface).Elem().Set(reflect.ValueOf(modObj))

	if inlineCfg.Children != nil || len(args) > 1 {
		if err := initInlineModule(modObj, globals, inlineCfg); err != nil {
			return err
		}
	}

	return nil
}
