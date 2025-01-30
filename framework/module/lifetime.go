/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2025 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

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

package module

import (
	"fmt"

	"github.com/foxcpp/maddy/framework/log"
)

// LifetimeModule is a stateful module that needs to have post-configuration
// startup and graceful shutdown functionality.
type LifetimeModule interface {
	Module
	Start() error
	Stop() error
}

type ReloadModule interface {
	Module
	Reload() error
}

type LifetimeTracker struct {
	logger    *log.Logger
	instances []*struct {
		mod     LifetimeModule
		started bool
	}
}

func (lt *LifetimeTracker) Add(mod LifetimeModule) {
	lt.instances = append(lt.instances, &struct {
		mod     LifetimeModule
		started bool
	}{mod: mod, started: false})
}

// StartAll calls Start for all registered LifetimeModule instances.
func (lt *LifetimeTracker) StartAll() error {
	for _, entry := range lt.instances {
		if entry.started {
			continue
		}

		if err := entry.mod.Start(); err != nil {
			lt.StopAll()
			return fmt.Errorf("failed to start module %v: %w",
				entry.mod.InstanceName(), err)
		}
		lt.logger.DebugMsg("module started",
			"mod_name", entry.mod.Name(), "inst_name", entry.mod.InstanceName())
		entry.started = true
	}
	return nil
}

func (lt *LifetimeTracker) ReloadAll() error {
	for _, entry := range lt.instances {
		if !entry.started {
			continue
		}

		rm, ok := entry.mod.(ReloadModule)
		if !ok {
			continue
		}

		if err := rm.Reload(); err != nil {
			lt.logger.Error("module reload failed", err,
				"mod_name", entry.mod.Name(), "inst_name", entry.mod.InstanceName())
			continue
		}

		lt.logger.DebugMsg("module reloaded",
			"mod_name", entry.mod.Name(), "inst_name", entry.mod.InstanceName())
	}
	return nil
}

// StopAll calls Stop for all registered LifetimeModule instances.
func (lt *LifetimeTracker) StopAll() error {
	for i := len(lt.instances) - 1; i >= 0; i-- {
		entry := lt.instances[i]

		if !entry.started {
			continue
		}

		if err := entry.mod.Stop(); err != nil {
			lt.logger.Error("module stop failed", err,
				"mod_name", entry.mod.Name(), "inst_name", entry.mod.InstanceName())
			continue
		}
		lt.logger.DebugMsg("module stopped",
			"mod_name", entry.mod.Name(), "inst_name", entry.mod.InstanceName())

		entry.started = false
	}
	return nil
}

func NewLifetime(log *log.Logger) *LifetimeTracker {
	return &LifetimeTracker{
		logger: log,
	}
}
