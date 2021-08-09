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
	"crypto/tls"
)

// TLSLoader interface is module interface that can be used to supply TLS
// certificates to TLS-enabled endpoints.
//
// The interface is intentionally kept simple, all configuration and parameters
// necessary are to be provided using conventional module configuration.
//
// If loader returns multiple certificate chains - endpoint will serve them
// based on SNI matching.
//
// Note that loading function will be called for each connections - it is
// highly recommended to cache parsed form.
//
// Modules implementing this interface should be registered with prefix
// "tls.loader." in name.
type TLSLoader interface {
	ConfigureTLS(c *tls.Config) error
}
