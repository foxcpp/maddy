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
	globals := config.NewMap(nil, &config.Node{Children: cfg})
	globals.String("hostname", false, false, "", nil)
	globals.String("statedir", false, false, "", nil)
	globals.String("libexecdir", false, false, "", nil)
	globals.Custom("tls", false, false, nil, tlsDirective, nil)
	globals.Custom("log", false, false, defaultLogOutput, logOutput, &log.DefaultLogger.Out)
	globals.Bool("debug", false, &log.DefaultLogger.Debug)
	globals.AllowUnknown()
	unmatched, err := globals.Process()
	if err != nil {
		return err
	}

	for _, block := range unmatched {
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

		if module.HasInstance(instName) {
			return fmt.Errorf("%s:%d: module instance named %s already exists", block.File, block.Line, instName)
		}

		log.Debugln("module create", modName, instName)
		inst, err := factory(modName, instName)
		if err != nil {
			return err
		}

		block := block
		module.RegisterInstance(inst, config.NewMap(globals.Values, &block))
		instances[instName] = modInfo{instance: inst, cfg: block}
	}

	for _, inst := range instances {
		if module.Initialized[inst.instance.InstanceName()] {
			log.Debugln("module init", inst.instance.Name(), inst.instance.InstanceName(), "skipped because it was lazily initialized before")
			continue
		}

		module.Initialized[inst.instance.InstanceName()] = true
		log.Debugln("module init", inst.instance.Name(), inst.instance.InstanceName())
		if err := inst.instance.Init(config.NewMap(globals.Values, &inst.cfg)); err != nil {
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
			if err := closer.Close(); err != nil {
				log.Printf("module %s (%s) close failed: %v", inst.instance.Name(), inst.instance.InstanceName(), err)
			}
		}
	}

	return nil
}
