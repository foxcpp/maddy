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

package module

import (
	"errors"

	"github.com/foxcpp/maddy/framework/log"
)

var (
	ErrInstanceNameDuplicate = errors.New("instance name already registered")
	ErrInstanceUnknown       = errors.New("no such instance registered")
)

type registryEntry struct {
	Mod      Module
	LazyInit func() error
}

type Registry struct {
	logger      log.Logger
	instances   map[string]registryEntry
	initialized map[string]struct{}
	started     map[string]struct{}
	aliases     map[string]string
}

func NewRegistry(log log.Logger) *Registry {
	return &Registry{
		logger:      log,
		instances:   make(map[string]registryEntry),
		initialized: make(map[string]struct{}),
		started:     make(map[string]struct{}),
		aliases:     make(map[string]string),
	}
}

// Register adds not-initialized (configured) module into registry.
//
// lazyInit function will be called on first request to get the module from
// registry.
func (r *Registry) Register(mod Module, lazyInit func() error) error {
	instName := mod.InstanceName()
	if instName == "" {
		panic("module with empty instance name cannot be added to the registry")
	}

	_, ok := r.instances[instName]
	if ok {
		return ErrInstanceNameDuplicate
	}

	r.instances[instName] = registryEntry{
		Mod:      mod,
		LazyInit: lazyInit,
	}
	return nil
}

func (r *Registry) AddAlias(instanceName string, alias string) error {
	if instanceName == "" {
		panic("cannot add an alias for empty instance name")
	}
	if alias == "" {
		panic("cannot add an empty alias")
	}
	_, ok := r.aliases[alias]
	if ok {
		return ErrInstanceNameDuplicate
	}
	_, ok = r.instances[instanceName]
	if ok {
		return ErrInstanceNameDuplicate
	}

	r.aliases[alias] = instanceName
	return nil
}

func (r *Registry) ensureInitialized(name string, entry *registryEntry) error {
	_, ok := r.initialized[name]
	if ok {
		return nil
	}
	if entry.LazyInit == nil {
		return nil
	}

	r.logger.DebugMsg("module configure",
		"mod_name", entry.Mod.Name(), "inst_name", entry.Mod.InstanceName())
	err := entry.LazyInit()
	if err != nil {
		return err
	}
	r.initialized[name] = struct{}{}

	return nil
}

func (r *Registry) Get(name string) (Module, error) {
	if name == "" {
		panic("cannot get module with empty name")
	}
	aliasedName := r.aliases[name]
	if aliasedName != "" {
		name = aliasedName
	}

	mod, ok := r.instances[name]
	if !ok {
		return nil, ErrInstanceUnknown
	}

	if err := r.ensureInitialized(name, &mod); err != nil {
		return nil, err
	}

	return mod.Mod, nil
}

func (r *Registry) NotInitialized() []Module {
	notinit := make([]Module, 0, len(r.instances)-len(r.initialized))
	for name, mod := range r.instances {
		if _, ok := r.initialized[name]; ok {
			continue
		}
		notinit = append(notinit, mod.Mod)
	}
	return notinit
}
