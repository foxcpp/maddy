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

// Package module contains modules registry and interfaces implemented
// by modules.
//
// Interfaces are placed here to prevent circular dependencies.
//
// Each interface required by maddy for operation is provided by some object
// called "module".  This includes authentication, storage backends, DKIM,
// email filters, etc.  Each module may serve multiple functions. I.e. it can
// be IMAP storage backend, SMTP downstream and authentication provider at the
// same moment.
//
// Each module gets its own unique name (sql for go-imap-sql, proxy for
// proxy module, local for local delivery perhaps, etc). Each module instance
// also can have its own unique name can be used to refer to it in
// configuration.
package module

import (
	"github.com/foxcpp/maddy/framework/config"
)

// Module is the interface implemented by all maddy module instances.
type Module interface {
	Configure(inlineArgs []string, config *config.Map) error

	// Name method reports module name.
	//
	// It is used to reference module in the configuration and in logs.
	Name() string

	// InstanceName method reports unique name of this module instance or empty
	// string if module instance is unnamed.
	InstanceName() string
}
