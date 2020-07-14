package module

import (
	"fmt"
	"io"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/hooks"
	"github.com/foxcpp/maddy/framework/log"
)

var (
	instances = make(map[string]struct {
		mod Module
		cfg *config.Map
	})
	aliases = make(map[string]string)

	Initialized = make(map[string]bool)
)

// RegisterInstance adds module instance to the global registry.
//
// Instance name must be unique. Second RegisterInstance with same instance
// name will replace previous.
func RegisterInstance(inst Module, cfg *config.Map) {
	instances[inst.InstanceName()] = struct {
		mod Module
		cfg *config.Map
	}{inst, cfg}
}

// RegisterAlias creates an association between a certain name and instance name.
//
// After RegisterAlias, module.GetInstance(aliasName) will return the same
// result as module.GetInstance(instName).
func RegisterAlias(aliasName, instName string) {
	aliases[aliasName] = instName
}

func HasInstance(name string) bool {
	aliasedName := aliases[name]
	if aliasedName != "" {
		name = aliasedName
	}

	_, ok := instances[name]
	return ok
}

// GetInstance returns module instance from global registry, initializing it if
// necessary.
//
// Error is returned if module initialization fails or module instance does not
// exists.
func GetInstance(name string) (Module, error) {
	aliasedName := aliases[name]
	if aliasedName != "" {
		name = aliasedName
	}

	mod, ok := instances[name]
	if !ok {
		return nil, fmt.Errorf("unknown config block: %s", name)
	}

	// Break circular dependencies.
	if Initialized[name] {
		return mod.mod, nil
	}

	Initialized[name] = true
	if err := mod.mod.Init(mod.cfg); err != nil {
		return mod.mod, err
	}

	if closer, ok := mod.mod.(io.Closer); ok {
		hooks.AddHook(hooks.EventShutdown, func() {
			log.Debugf("close %s (%s)", mod.mod.Name(), mod.mod.InstanceName())
			if err := closer.Close(); err != nil {
				log.Printf("module %s (%s) close failed: %v", mod.mod.Name(), mod.mod.InstanceName(), err)
			}
		})
	}

	return mod.mod, nil
}
