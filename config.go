package maddy

import (
	"errors"
	"fmt"

	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/module"
)

/*
Config matchers for module interfaces.
*/

func authProvider(modName string) (module.AuthProvider, error) {
	modObj, err := module.GetInstance(modName)
	if err != nil {
		return nil, err
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
	modObj, err := module.GetInstance(modName)
	if err != nil {
		return nil, err
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
	mod, err := module.GetInstance(modName)
	if mod == nil {
		return nil, err
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
