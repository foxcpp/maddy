package module

import (
	"sync"
)

var modules = make(map[string]FuncNewModule)
var modulesLock sync.RWMutex

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

// GetMod returns module from global registry.
//
// Nil is returned if no module with specified name is registered.
func GetMod(name string) FuncNewModule {
	modulesLock.RLock()
	defer modulesLock.RUnlock()

	return modules[name]
}

var instances = make(map[string]Module)
var instancesLck sync.RWMutex

// Register adds module instance to global registry.
//
// Instnace name must be unique. Second RegisterInstance with same instance
// name will replace previous.
func RegisterInstance(inst Module) {
	instancesLck.Lock()
	defer instancesLck.Unlock()

	instances[inst.InstanceName()] = inst
}

// GetInstance returns module instance from global registry.
//
// Nil is returned if no module instance with specified name is registered.
func GetInstance(name string) Module {
	instancesLck.RLock()
	defer instancesLck.RUnlock()

	return instances[name]
}

// UnregisterInstance undoes effect of RegisterInstance, removing instance from
// global registry.
func UnregisterInstance(name string) {
	instancesLck.Lock()
	defer instancesLck.Unlock()

	delete(instances, name)
}
