package maddy

import (
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/module"
)

type modInfo struct {
	instance module.Module
	cfg      config.Node
}

func instancesFromConfig(globals map[string]interface{}, nodes []config.Node) ([]module.Module, error) {
	var (
		endpoints []modInfo
		mods      = make([]modInfo, 0, len(nodes))
	)

	for _, block := range nodes {
		var instName string
		var modAliases []string
		if len(block.Args) == 0 {
			instName = block.Name
		} else {
			instName = block.Args[0]
			modAliases = block.Args[1:]
		}

		modName := block.Name

		endpFactory := module.GetEndpoint(modName)
		if endpFactory != nil {
			inst, err := endpFactory(modName, block.Args)
			if err != nil {
				return nil, err
			}

			endpoints = append(endpoints, modInfo{instance: inst, cfg: block})
			continue
		}

		factory := module.Get(modName)
		if factory == nil {
			return nil, config.NodeErr(&block, "unknown module: %s", modName)
		}

		if module.HasInstance(instName) {
			return nil, config.NodeErr(&block, "config block named %s already exists", instName)
		}

		inst, err := factory(modName, instName, modAliases, nil)
		if err != nil {
			return nil, err
		}

		block := block
		module.RegisterInstance(inst, config.NewMap(globals, &block))
		for _, alias := range modAliases {
			if module.HasInstance(alias) {
				return nil, config.NodeErr(&block, "config block named %s already exists", alias)
			}
			module.RegisterAlias(alias, instName)
		}
		mods = append(mods, modInfo{instance: inst, cfg: block})
	}

	for _, endp := range endpoints {
		if err := endp.instance.Init(config.NewMap(globals, &endp.cfg)); err != nil {
			return nil, err
		}
	}

	// We initialize all non-endpoint modules defined at top-level
	// just for purpose of checking that they have a valid configuration.
	//
	// Modules that are actually used will be pulled by lazy initialization
	// logic during endpoint initialization.
	for _, inst := range mods {
		if module.Initialized[inst.instance.InstanceName()] {
			continue
		}

		log.Printf("%s (%s) is not used anywhere", inst.instance.InstanceName(), inst.instance.Name())

		module.Initialized[inst.instance.InstanceName()] = true
		if err := inst.instance.Init(config.NewMap(globals, &inst.cfg)); err != nil {
			return nil, err
		}
	}

	res := make([]module.Module, 0, len(mods)+len(endpoints))
	for _, endp := range endpoints {
		res = append(res, endp.instance)
	}
	for _, mod := range mods {
		res = append(res, mod.instance)
	}
	return res, nil
}
