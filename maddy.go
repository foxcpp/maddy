package maddy

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/log"
	"github.com/emersion/maddy/module"
)

type modInfo struct {
	instance module.Module
	cfg      config.Node
}

func Start(cfg []config.Node) error {
	instances := make(map[string]modInfo)
	globalCfg := make(map[string]config.Node)

	for _, block := range cfg {
		switch block.Name {
		case "tls", "hostname", "debug":
			globalCfg[block.Name] = block
			continue
		}
	}

	if _, ok := globalCfg["debug"]; ok {
		log.DefaultLogger.Debug = true
	}

	for _, block := range cfg {
		switch block.Name {
		case "hostname", "tls", "debug":
			continue
		}

		var instName string
		if len(block.Args) == 0 {
			instName = block.Name
		} else {
			instName = block.Args[0]
		}

		modName := block.Name

		factory := module.Get(modName)
		if factory == nil {
			return fmt.Errorf("%s:%d: unknown module: %s", block.File, block.Line, modName)
		}

		if mod := module.GetInstance(instName); mod != nil {
			return fmt.Errorf("%s:%d: module instance named %s already exists", block.File, block.Line, instName)
		}

		log.Debugln("module create", modName, instName)
		inst, err := factory(modName, instName)
		if err != nil {
			return err
		}

		module.RegisterInstance(inst)
		instances[instName] = modInfo{instance: inst, cfg: block}
	}

	addDefaultModule(instances, "default", createDefaultStorage, defaultStorageConfig)
	addDefaultModule(instances, "default_remote_delivery", createDefaultRemoteDelivery, nil)

	for _, inst := range instances {
		log.Debugln("module init", inst.instance.Name(), inst.instance.InstanceName())
		if err := inst.instance.Init(globalCfg, inst.cfg); err != nil {
			return err
		}
	}

	sig := make(chan os.Signal, 5)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT)

	s := <-sig
	log.Printf("signal received (%v), next signal will force immediate shutdown.", s)
	go func() {
		s := <-sig
		log.Printf("forced shutdown due to signal (%v)!", s)
		os.Exit(1)
	}()

	for _, inst := range instances {
		if closer, ok := inst.instance.(io.Closer); ok {
			log.Debugln("clean-up for module", inst.instance.Name(), inst.instance.InstanceName())
			closer.Close()
		}
	}

	return nil
}

func addDefaultModule(insts map[string]modInfo, name string, factory func(string) (module.Module, error), cfgFactory func(string) config.Node) {
	if _, ok := insts[name]; !ok {
		if mod, err := factory(name); err != nil {
			log.Printf("failed to register %s: %v", name, err)
		} else {
			log.Debugf("module create %s %s (built-in)", mod.Name(), name)
			module.RegisterInstance(mod)
			info := modInfo{instance: mod}
			if cfgFactory != nil {
				info.cfg = cfgFactory(name)
			}
			insts[name] = info
		}
	} else {
		log.Debugf("module create %s (built-in) skipped because user-defined exists", name)
	}
}
