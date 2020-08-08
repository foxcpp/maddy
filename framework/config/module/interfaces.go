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

package modconfig

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
)

func MessageCheck(globals map[string]interface{}, args []string, block config.Node) (module.Check, error) {
	var check module.Check
	if err := ModuleFromNode("check", args, block, globals, &check); err != nil {
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
func DeliveryDirective(m *config.Map, node config.Node) (interface{}, error) {
	return DeliveryTarget(m.Globals, node.Args, node)
}

func DeliveryTarget(globals map[string]interface{}, args []string, block config.Node) (module.DeliveryTarget, error) {
	var target module.DeliveryTarget
	if err := ModuleFromNode("target", args, block, globals, &target); err != nil {
		return nil, err
	}
	return target, nil
}

func MsgModifier(globals map[string]interface{}, args []string, block config.Node) (module.Modifier, error) {
	var check module.Modifier
	if err := ModuleFromNode("modify", args, block, globals, &check); err != nil {
		return nil, err
	}
	return check, nil
}

func IMAPFilter(globals map[string]interface{}, args []string, block config.Node) (module.IMAPFilter, error) {
	var filter module.IMAPFilter
	if err := ModuleFromNode("imap.filter", args, block, globals, &filter); err != nil {
		return nil, err
	}
	return filter, nil
}

func StorageDirective(m *config.Map, node config.Node) (interface{}, error) {
	var backend module.Storage
	if err := ModuleFromNode("storage", node.Args, node, m.Globals, &backend); err != nil {
		return nil, err
	}
	return backend, nil
}

func TableDirective(m *config.Map, node config.Node) (interface{}, error) {
	var tbl module.Table
	if err := ModuleFromNode("table", node.Args, node, m.Globals, &tbl); err != nil {
		return nil, err
	}
	return tbl, nil
}
