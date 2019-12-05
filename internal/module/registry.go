package module

import (
	"sync"
)

var (
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
