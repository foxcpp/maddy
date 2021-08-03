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

package dns

import (
	"context"
	"net"
	"time"
)

var overrideServ string

// override globally overrides the used DNS server address with one provided.
// This function is meant only for testing. It should be called before any modules are
// initialized to have full effect.
//
// The server argument is in form of "IP:PORT". It is expected that the server
// will be available both using TCP and UDP on the same port.
func override(server string) {
	net.DefaultResolver.PreferGo = true
	net.DefaultResolver.Dial = func(ctx context.Context, network, _ string) (net.Conn, error) {
		dialer := net.Dialer{
			// This is localhost, it is either running or not. Fail quickly if
			// we can't connect.
			Timeout: 1 * time.Second,
		}

		switch network {
		case "udp", "udp4", "udp6":
			return dialer.DialContext(ctx, "udp4", server)
		case "tcp", "tcp4", "tcp6":
			return dialer.DialContext(ctx, "tcp4", server)
		default:
			panic("OverrideDNS.Dial: unknown network")
		}
	}
}
