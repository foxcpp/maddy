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

package module

import (
	"sync"

	"github.com/foxcpp/maddy/framework/log"
)

var (
	// NoRun makes sure modules do not start any bacground tests.
	//
	// If it set - modules should not perform any actual work and should stop
	// once the configuration is read and verified to be correct.
	// TODO: Replace it with separation of Init and Run at interface level.
	NoRun = false

	modules     = make(map[string]FuncNewModule)
	endpoints   = make(map[string]FuncNewEndpoint)
	modulesLock sync.RWMutex
)

// Register adds module factory function to global registry.
//
// name must be unique. Register will panic if module with specified name
// already exists in registry.
//
// You probably want to call this function from func init() of module package.
func Register(name string, factory FuncNewModule) {
	modulesLock.Lock()
	defer modulesLock.Unlock()

	if _, ok := modules[name]; ok {
		panic("Register: module with specified name is already registered: " + name)
	}

	modules[name] = factory
}

// RegisterDeprecated adds module factory function to global registry.
//
// It prints warning to the log about name being deprecated and suggests using
// a new name.
func RegisterDeprecated(name, newName string, factory FuncNewModule) {
	Register(name, func(modName, instName string, aliases, inlineArgs []string) (Module, error) {
		log.Printf("module initialized via deprecated name %s, %s should be used instead; deprecated name may be removed in the next version", name, newName)
		return factory(modName, instName, aliases, inlineArgs)
	})
}

// Get returns module from global registry.
//
// This function does not return endpoint-type modules, use GetEndpoint for
// that.
// Nil is returned if no module with specified name is registered.
func Get(name string) FuncNewModule {
	modulesLock.RLock()
	defer modulesLock.RUnlock()

	return modules[name]
}

// GetEndpoints returns an endpoint module from global registry.
//
// Nil is returned if no module with specified name is registered.
func GetEndpoint(name string) FuncNewEndpoint {
	modulesLock.RLock()
	defer modulesLock.RUnlock()

	return endpoints[name]
}

// RegisterEndpoint registers an endpoint module.
//
// See FuncNewEndpoint for information about
// differences of endpoint modules from regular modules.
func RegisterEndpoint(name string, factory FuncNewEndpoint) {
	modulesLock.Lock()
	defer modulesLock.Unlock()

	if _, ok := endpoints[name]; ok {
		panic("Register: module with specified name is already registered: " + name)
	}

	endpoints[name] = factory
}
