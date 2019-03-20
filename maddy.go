package maddy

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/module"
)

func Start(cfg []config.Node) error {
	instances := make([]module.Module, 0, len(cfg))
	for _, block := range cfg {
		var instName string
		if len(block.Args) == 0 {
			instName = block.Name
		} else {
			instName = block.Args[0]
		}

		modName := block.Name

		factory := module.GetMod(modName)
		if factory == nil {
			return fmt.Errorf("unknown module: %s", modName)
		}

		if mod := module.GetInstance(instName); mod != nil {
			if !strings.HasPrefix(instName, "default") {
				return fmt.Errorf("module instance named %s already exists", instName)
			}

			// Clean up default module before replacing it.
			if closer, ok := mod.(io.Closer); ok {
				closer.Close()
			}
		}

		inst, err := factory(instName, block)
		if err != nil {
			return fmt.Errorf("module instance %s initialization failed: %v", instName, err)
		}

		module.RegisterInstance(inst)
		instances = append(instances, inst)
	}

	sig := make(chan os.Signal, 5)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT)

	s := <-sig
	log.Printf("signal received (%v), next signal will force immediate shutdown.\n", s)
	go func() {
		s := <-sig
		log.Printf("forced shutdown due to signal (%v)!\n", s)
		os.Exit(1)
	}()

	for _, inst := range instances {
		if closer, ok := inst.(io.Closer); ok {
			closer.Close()
		}
	}

	return nil
}

func authProvider(args []string) (module.AuthProvider, error) {
	if len(args) != 1 {
		return nil, errors.New("auth: expected 1 argument")
	}

	authName := args[0]
	authMod := module.GetInstance(authName)
	if authMod == nil {
		return nil, fmt.Errorf("unknown auth. provider instance: %s", authName)
	}

	provider, ok := authMod.(module.AuthProvider)
	if !ok {
		return nil, fmt.Errorf("module %s doesn't implements auth. provider interface", authMod.Name())
	}
	return provider, nil
}

func storageBackend(args []string) (module.Storage, error) {
	if len(args) != 1 {
		return nil, errors.New("storage: expected 1 argument")
	}

	authName := args[0]
	authMod := module.GetInstance(authName)
	if authMod == nil {
		return nil, fmt.Errorf("unknown storage backend instance: %s", authName)
	}

	provider, ok := authMod.(module.Storage)
	if !ok {
		return nil, fmt.Errorf("module %s doesn't implements storage interface", authMod.Name())
	}
	return provider, nil
}

func deliveryTarget(args []string) (module.DeliveryTarget, error) {
	if len(args) != 1 {
		return nil, errors.New("delivery: expected 1 argument")
	}

	modName := args[0]
	mod := module.GetInstance(modName)
	if mod == nil {
		return nil, fmt.Errorf("unknown delivery target instance: %s", modName)
	}

	target, ok := mod.(module.DeliveryTarget)
	if !ok {
		return nil, fmt.Errorf("module %s doesn't implements delivery target interface", mod.Name())
	}
	return target, nil
}
