/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package ctl

import (
	"errors"
	"fmt"
	"os"

	"github.com/foxcpp/maddy"
	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/updatepipe"
	"github.com/urfave/cli/v2"
)

func closeIfNeeded(i any) {
	if c, ok := i.(container.LifetimeModule); ok {
		if err := c.Stop(); err != nil {
			log.DefaultLogger.Error("failed to stop module", err)
		}
	}
}

type managedStorage struct {
	module.ManageableStorage
	started bool
}

func (m *managedStorage) Close() error {
	if !m.started {
		return nil
	}
	if lm, ok := m.ManageableStorage.(container.LifetimeModule); ok {
		return lm.Stop()
	}
	return nil
}

type managedUserDB struct {
	module.PlainUserDB
	started bool
}

func (m *managedUserDB) Close() error {
	if !m.started {
		return nil
	}
	if lm, ok := m.PlainUserDB.(container.LifetimeModule); ok {
		return lm.Stop()
	}
	return nil
}

func getCfgBlockModule(ctx *cli.Context) (*container.C, module.Module, error) {
	cfgPath := ctx.String("config")
	if cfgPath == "" {
		return nil, nil, cli.Exit("Error: config is required", 2)
	}

	c := container.New()
	container.Global = c

	cfg, err := maddy.ReadConfig(cfgPath)
	if err != nil {
		return nil, nil, cli.Exit(fmt.Sprintf("Error: failed to open config: %v", err), 2)
	}

	globals, cfgNodes, err := maddy.ReadGlobals(c, cfg)
	if err != nil {
		return nil, nil, err
	}

	// For CLI management we force-rollback configured logger and consider only
	// --log so messages relevant to command execution will go where admin would
	// see them.
	c.DefaultLogger.Out = log.DefaultLogger.Out

	if err := maddy.InitDirs(c); err != nil {
		return nil, nil, err
	}

	err = maddy.RegisterModules(c, globals, cfgNodes)
	if err != nil {
		return nil, nil, err
	}

	cfgBlock := ctx.String("cfg-block")
	if cfgBlock == "" {
		return nil, nil, cli.Exit("Error: cfg-block is required", 2)
	}

	mod, err := c.Modules.Get(cfgBlock)
	if err != nil {
		if errors.Is(err, container.ErrInstanceUnknown) {
			return nil, nil, cli.Exit(fmt.Sprintf("Error: unknown configuration block: %s", cfgBlock), 2)
		}
		return nil, nil, err
	}

	return c, mod, nil
}

func openStorage(ctx *cli.Context) (module.Storage, error) {
	_, mod, err := getCfgBlockModule(ctx)
	if err != nil {
		return nil, err
	}

	storage, ok := mod.(module.Storage)
	if !ok {
		return nil, cli.Exit(fmt.Sprintf("Error: configuration block %s is not an IMAP storage", ctx.String("cfg-block")), 2)
	}

	started := false
	if lt, ok := storage.(container.LifetimeModule); ok {
		if err := lt.Start(); err != nil {
			return nil, err
		}
		started = true
	}

	if updStore, ok := mod.(updatepipe.Backend); ok {
		if err := updStore.EnableUpdatePipe(updatepipe.ModePush); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "Failed to initialize update pipe, do not remove messages from mailboxes open by clients: %v\n", err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "No update pipe support, do not remove messages from mailboxes open by clients\n")
	}

	if started {
		if ms, ok := storage.(module.ManageableStorage); ok {
			return &managedStorage{ManageableStorage: ms, started: started}, nil
		}
	}
	return storage, nil
}

func openUserDB(ctx *cli.Context) (module.PlainUserDB, error) {
	_, mod, err := getCfgBlockModule(ctx)
	if err != nil {
		return nil, err
	}

	userDB, ok := mod.(module.PlainUserDB)
	if !ok {
		return nil, cli.Exit(fmt.Sprintf("Error: configuration block %s is not a local credentials store", ctx.String("cfg-block")), 2)
	}

	started := false
	if lt, ok := userDB.(container.LifetimeModule); ok {
		if err := lt.Start(); err != nil {
			return nil, err
		}
		started = true
	}

	if started {
		return &managedUserDB{PlainUserDB: userDB, started: started}, nil
	}
	return userDB, nil
}
