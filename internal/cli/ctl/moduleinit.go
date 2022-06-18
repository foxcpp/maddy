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
	"io"
	"os"

	"github.com/foxcpp/maddy"
	parser "github.com/foxcpp/maddy/framework/cfgparser"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/hooks"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/updatepipe"
	"github.com/urfave/cli/v2"
)

func closeIfNeeded(i interface{}) {
	if c, ok := i.(io.Closer); ok {
		c.Close()
	}
}

func getCfgBlockModule(ctx *cli.Context) (map[string]interface{}, *maddy.ModInfo, error) {
	cfgPath := ctx.String("config")
	if cfgPath == "" {
		return nil, nil, cli.Exit("Error: config is required", 2)
	}
	cfgFile, err := os.Open(cfgPath)
	if err != nil {
		return nil, nil, cli.Exit(fmt.Sprintf("Error: failed to open config: %v", err), 2)
	}
	defer cfgFile.Close()
	cfgNodes, err := parser.Read(cfgFile, cfgFile.Name())
	if err != nil {
		return nil, nil, cli.Exit(fmt.Sprintf("Error: failed to parse config: %v", err), 2)
	}

	globals, cfgNodes, err := maddy.ReadGlobals(cfgNodes)
	if err != nil {
		return nil, nil, err
	}

	if err := maddy.InitDirs(); err != nil {
		return nil, nil, err
	}

	module.NoRun = true
	_, mods, err := maddy.RegisterModules(globals, cfgNodes)
	if err != nil {
		return nil, nil, err
	}
	defer hooks.RunHooks(hooks.EventShutdown)

	cfgBlock := ctx.String("cfg-block")
	if cfgBlock == "" {
		return nil, nil, cli.Exit("Error: cfg-block is required", 2)
	}
	var mod maddy.ModInfo
	for _, m := range mods {
		if m.Instance.InstanceName() == cfgBlock {
			mod = m
			break
		}
	}
	if mod.Instance == nil {
		return nil, nil, cli.Exit(fmt.Sprintf("Error: unknown configuration block: %s", cfgBlock), 2)
	}

	return globals, &mod, nil
}

func openStorage(ctx *cli.Context) (module.Storage, error) {
	globals, mod, err := getCfgBlockModule(ctx)
	if err != nil {
		return nil, err
	}

	storage, ok := mod.Instance.(module.Storage)
	if !ok {
		return nil, cli.Exit(fmt.Sprintf("Error: configuration block %s is not an IMAP storage", ctx.String("cfg-block")), 2)
	}

	if err := mod.Instance.Init(config.NewMap(globals, mod.Cfg)); err != nil {
		return nil, fmt.Errorf("Error: module initialization failed: %w", err)
	}

	if updStore, ok := mod.Instance.(updatepipe.Backend); ok {
		if err := updStore.EnableUpdatePipe(updatepipe.ModePush); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "Failed to initialize update pipe, do not remove messages from mailboxes open by clients: %v\n", err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "No update pipe support, do not remove messages from mailboxes open by clients\n")
	}

	return storage, nil
}

func openUserDB(ctx *cli.Context) (module.PlainUserDB, error) {
	globals, mod, err := getCfgBlockModule(ctx)
	if err != nil {
		return nil, err
	}

	userDB, ok := mod.Instance.(module.PlainUserDB)
	if !ok {
		return nil, cli.Exit(fmt.Sprintf("Error: configuration block %s is not a local credentials store", ctx.String("cfg-block")), 2)
	}

	if err := mod.Instance.Init(config.NewMap(globals, mod.Cfg)); err != nil {
		return nil, fmt.Errorf("Error: module initialization failed: %w", err)
	}

	return userDB, nil
}
