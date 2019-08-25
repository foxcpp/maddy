package module

import (
	"fmt"

	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/log"
)

var (
	instances = make(map[string]struct {
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
	instances[inst.InstanceName()] = struct {
		mod Module
		cfg *config.Map
	}{inst, cfg}
}

func HasInstance(name string) bool {
	_, ok := instances[name]
	return ok
}

// GetInstance returns module instance from global registry, initializing it if
// necessary.
//
// Error is returned if module initialization fails or module instance does not
// exists.
func GetInstance(name string) (Module, error) {
	mod, ok := instances[name]
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
