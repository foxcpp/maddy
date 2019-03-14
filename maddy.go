package maddy

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/emersion/maddy/module"
	"github.com/mholt/caddy/caddyfile"
)

var Directives []string

func Start(cfg []caddyfile.ServerBlock) error {
	var instances []module.Module
	for _, block := range cfg {
		if len(block.Keys) != 2 {
			return fmt.Errorf("wanted 2 keys in module instance definition, got %d (%v)", len(block.Keys), block.Keys)
		}

		modName := block.Keys[0]
		instName := block.Keys[1]

		factory := module.GetMod(modName)
		if factory == nil {
			return fmt.Errorf("unknown module: %s", modName)
		}

		if module.GetInstance(instName) != nil {
			return fmt.Errorf("module instance named %s already exists", instName)
		}

		inst, err := factory(instName, block.Tokens)
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
	module.WaitGroup.Wait()

	return nil
}

func authProvider(tokens []caddyfile.Token) (module.AuthProvider, error) {
	var authName string
	var ok bool
	d := caddyfile.NewDispenserTokens("", tokens)
	d.Next()
	args := d.RemainingArgs()
	if len(args) != 1 {
		return nil, errors.New("auth: expected 1 argument")
	}

	authName = args[0]
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

func storageBackend(tokens []caddyfile.Token) (module.Storage, error) {
	var authName string
	var ok bool
	d := caddyfile.NewDispenserTokens("", tokens)
	d.Next()
	args := d.RemainingArgs()
	if len(args) != 1 {
		return nil, errors.New("storage: expected 1 argument")
	}

	authName = args[0]
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

func oneStringValue(tokens []caddyfile.Token) string {
	d := caddyfile.NewDispenserTokens("", tokens)
	d.Next()
	args := d.RemainingArgs()
	if len(args) == 0 {
		return ""
	}
	return args[0]
}
