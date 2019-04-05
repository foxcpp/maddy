package module

import (
	"fmt"

	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/log"
)

var (
	Instances = make(map[string]struct {
		mod Module
		cfg *config.Map
	})

	Initialized = make(map[string]bool)
)

// Register adds module instance to the global registry.
//
// Instnace name must be unique. Second RegisterInstance with same instance
// name will replace previous.
func RegisterInstance(inst Module, cfg *config.Map) {
	Instances[inst.InstanceName()] = struct {
		mod Module
		cfg *config.Map
	}{inst, cfg}
}

// GetUninitedInstance returns module instance from global registry.
//
// Nil is returned if no module instance with specified name is Instances.
//
// Note that this function may return uninitialized module. If you need to ensure
// that it is safe to interact with module instance - use GetInstance instead.
func GetUninitedInstance(name string) Module {
	mod, ok := Instances[name]
	if !ok {
		return nil
	}

	return mod.mod
}

// GetInstance returns module instance from global registry, initializing it if
// necessary.
//
// Error is returned if module initialization fails or module instance does not
// exists.
func GetInstance(name string) (Module, error) {
	mod, ok := Instances[name]
	if !ok {
		return nil, fmt.Errorf("unknown module instance: %s", name)
	}

	// Break circular dependencies.
	if Initialized[name] {
		log.Debugln("module init", mod.mod.Name(), name, "(lazy init skipped)")
		return mod.mod, nil
	}

	log.Debugln("module init", mod.mod.Name(), name, "(lazy)")
	Initialized[name] = true
	if err := mod.mod.Init(mod.cfg); err != nil {
		return mod.mod, err
	}

	return mod.mod, nil
}
