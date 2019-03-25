package maddy

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/module"
)

/*
Config matchers for module interfaces.
*/

func authProvider(modName string) (module.AuthProvider, error) {
	modObj := module.GetInstance(modName)
	if modObj == nil {
		return nil, fmt.Errorf("unknown auth. provider instance: %s", modName)
	}

	provider, ok := modObj.(module.AuthProvider)
	if !ok {
		return nil, fmt.Errorf("module %s doesn't implements auth. provider interface", modObj.Name())
	}
	return provider, nil
}

func authDirective(m *config.Map, node *config.Node) (interface{}, error) {
	if len(node.Args) != 1 {
		return nil, m.MatchErr("expected 1 argument")
	}
	if len(node.Children) != 0 {
		return nil, m.MatchErr("can't declare block here")
	}

	modObj, err := authProvider(node.Args[0])
	if err != nil {
		return nil, m.MatchErr("%s", err.Error())
	}

	return modObj, nil
}

func storageBackend(modName string) (module.Storage, error) {
	modObj := module.GetInstance(modName)
	if modObj == nil {
		return nil, fmt.Errorf("unknown storage backend instance: %s", modName)
	}

	backend, ok := modObj.(module.Storage)
	if !ok {
		return nil, fmt.Errorf("module %s doesn't implements storage interface", modObj.Name())
	}
	return backend, nil
}

func storageDirective(m *config.Map, node *config.Node) (interface{}, error) {
	if len(node.Args) != 1 {
		return nil, m.MatchErr("expected 1 argument")
	}
	if len(node.Children) != 0 {
		return nil, m.MatchErr("can't declare block here")
	}

	modObj, err := storageBackend(node.Args[0])
	if err != nil {
		return nil, m.MatchErr("%s", err.Error())
	}

	return modObj, nil
}

func deliveryTarget(modName string) (module.DeliveryTarget, error) {
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

func deliveryDirective(m *config.Map, node *config.Node) (interface{}, error) {
	if len(node.Args) != 1 {
		return nil, m.MatchErr("expected 1 argument")
	}
	if len(node.Children) != 0 {
		return nil, m.MatchErr("can't declare block here")
	}

	modObj, err := deliveryTarget(node.Args[0])
	if err != nil {
		return nil, m.MatchErr("%s", err.Error())
	}
	return modObj, nil
}

func defaultAuthProvider() (interface{}, error) {
	res, err := authProvider("default_auth")
	if err != nil {
		res, err = authProvider("default")
		if err != nil {
			return nil, errors.New("missing default auth. provider, must set custom")
		}
	}
	return res, nil
}

func defaultStorage() (interface{}, error) {
	res, err := storageBackend("default_storage")
	if err != nil {
		res, err = storageBackend("default")
		if err != nil {
			return nil, errors.New("missing default storage backend, must set custom")
		}
	}
	return res, nil
}

func errorsDirective(m *config.Map, node *config.Node) (interface{}, error) {
	if len(node.Args) != 1 {
		return nil, m.MatchErr("expected 1 argument")
	}
	if len(node.Children) != 0 {
		return nil, m.MatchErr("can't declare block here")
	}

	output := node.Args[0]
	var w io.Writer
	switch output {
	case "off":
		w = ioutil.Discard
	case "stdout":
		w = os.Stdout
	case "stderr":
		w = os.Stderr
	default:
		f, err := os.OpenFile(output, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
		if err != nil {
			return nil, err
		}
		w = f
	}

	return log.New(w, "imap ", log.LstdFlags), nil
}
