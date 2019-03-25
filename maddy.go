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

func Start(cfg []config.Node) error {
	instances := make([]module.Module, 0, len(cfg))
	globalCfg := make(map[string]config.Node)

	defaultPresent := false
	for _, block := range cfg {
		switch block.Name {
		case "tls", "hostname", "debug":
			globalCfg[block.Name] = block
			continue
		default:
			if len(block.Args) != 0 && block.Args[0] == "default" {
				defaultPresent = true
			}
		}
	}

	if !defaultPresent {
		initDefaultStorage(globalCfg)
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

		factory := module.GetMod(modName)
		if factory == nil {
			return fmt.Errorf("%s:%d: unknown module: %s", block.File, block.Line, modName)
		}

		if mod := module.GetInstance(instName); mod != nil {
			return fmt.Errorf("%s:%d: module instance named %s already exists", block.File, block.Line, instName)
		}

		log.Debugln("init for module", modName, instName)
		inst, err := factory(instName, globalCfg, block)
		if err != nil {
			return err
		}

		module.RegisterInstance(inst)
		instances = append(instances, inst)
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
		if closer, ok := inst.(io.Closer); ok {
			log.Debugln("clean-up for module", inst.Name(), inst.InstanceName())
			closer.Close()
		}
	}

	return nil
}
