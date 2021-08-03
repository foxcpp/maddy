/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

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
	"io"
	"reflect"
	"strings"

	parser "github.com/foxcpp/maddy/framework/cfgparser"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/hooks"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
)

// createInlineModule is a helper function for config matchers that can create inline modules.
func createInlineModule(preferredNamespace, modName string, args []string) (module.Module, error) {
	var newMod module.FuncNewModule
	originalModName := modName

	// First try to extend the name with preferred namespace unless the name
	// already contains it.
	if !strings.Contains(modName, ".") && preferredNamespace != "" {
		modName = preferredNamespace + "." + modName
		newMod = module.Get(modName)
	}

	// Then try global namespace for compatibility and complex modules.
	if newMod == nil {
		newMod = module.Get(originalModName)
	}

	// Bail if both failed.
	if newMod == nil {
		return nil, fmt.Errorf("unknown module: %s (namespace: %s)", originalModName, preferredNamespace)
	}

	return newMod(modName, "", nil, args)
}

// initInlineModule constructs "faked" config tree and passes it to module
// Init function to make it look like it is defined at top-level.
//
// args must contain at least one argument, otherwise initInlineModule panics.
func initInlineModule(modObj module.Module, globals map[string]interface{}, block config.Node) error {
	err := modObj.Init(config.NewMap(globals, block))
	if err != nil {
		return err
	}

	if closer, ok := modObj.(io.Closer); ok {
		hooks.AddHook(hooks.EventShutdown, func() {
			log.Debugf("close %s (%s)", modObj.Name(), modObj.InstanceName())
			if err := closer.Close(); err != nil {
				log.Printf("module %s (%s) close failed: %v", modObj.Name(), modObj.InstanceName(), err)
			}
		})
	}

	return nil
}

// ModuleFromNode does all work to create or get existing module object with a certain type.
// It is not used by top-level module definitions, only for references from other
// modules configuration blocks.
//
// inlineCfg should contain configuration directives for inline declarations.
// args should contain values that are used to create module.
// It should be either module name + instance name or just module name. Further extensions
// may add other string arguments (currently, they can be accessed by module instances
// as inlineArgs argument to constructor).
//
// It checks using reflection whether it is possible to store a module object into modObj
// pointer (e.g. it implements all necessary interfaces) and stores it if everything is fine.
// If module object doesn't implement necessary module interfaces - error is returned.
// If modObj is not a pointer, ModuleFromNode panics.
//
// preferredNamespace is used as an implicit prefix for module name lookups.
// Module with name preferredNamespace + "." + args[0] will be preferred over just args[0].
// It can be omitted.
func ModuleFromNode(preferredNamespace string, args []string, inlineCfg config.Node, globals map[string]interface{}, moduleIface interface{}) error {
	if len(args) == 0 {
		return parser.NodeErr(inlineCfg, "at least one argument is required")
	}

	referenceExisting := strings.HasPrefix(args[0], "&")

	var modObj module.Module
	var err error
	if referenceExisting {
		if len(args) != 1 || inlineCfg.Children != nil {
			return parser.NodeErr(inlineCfg, "exactly one argument is required to use existing config block")
		}
		modObj, err = module.GetInstance(args[0][1:])
		log.Debugf("%s:%d: reference %s", inlineCfg.File, inlineCfg.Line, args[0])
	} else {
		log.Debugf("%s:%d: new module %s %v", inlineCfg.File, inlineCfg.Line, args[0], args[1:])
		modObj, err = createInlineModule(preferredNamespace, args[0], args[1:])
	}
	if err != nil {
		return err
	}

	// NOTE: This will panic if moduleIface is not a pointer.
	modIfaceType := reflect.TypeOf(moduleIface).Elem()
	modObjType := reflect.TypeOf(modObj)

	if modIfaceType.Kind() == reflect.Interface {
		// Case for assignment to module interface type.
		if !modObjType.Implements(modIfaceType) && !modObjType.AssignableTo(modIfaceType) {
			return parser.NodeErr(inlineCfg, "module %s (%s) doesn't implement %v interface", modObj.Name(), modObj.InstanceName(), modIfaceType)
		}
	} else if !modObjType.AssignableTo(modIfaceType) {
		// Case for assignment to concrete module type. Used in "module groups".
		return parser.NodeErr(inlineCfg, "module %s (%s) is not %v", modObj.Name(), modObj.InstanceName(), modIfaceType)
	}

	reflect.ValueOf(moduleIface).Elem().Set(reflect.ValueOf(modObj))

	if !referenceExisting {
		if err := initInlineModule(modObj, globals, inlineCfg); err != nil {
			return err
		}
	}

	return nil
}

// GroupFromNode provides a special kind of ModuleFromNode syntax that allows
// to omit the module name when defining inine configuration.  If it is not
// present, name in defaultModule is used.
func GroupFromNode(defaultModule string, args []string, inlineCfg config.Node, globals map[string]interface{}, moduleIface interface{}) error {
	if len(args) == 0 {
		args = append(args, defaultModule)
	}
	return ModuleFromNode("", args, inlineCfg, globals, moduleIface)
}
