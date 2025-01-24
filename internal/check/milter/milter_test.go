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

package milter

import (
	"testing"

	"github.com/foxcpp/maddy/framework/config"
)

func TestAcceptValidEndpoints(t *testing.T) {
	for _, endpoint := range []string{
		"tcp://0.0.0.0:10025",
		"tcp://[::]:10025",
		"tcp:127.0.0.1:10025",
		"unix://path",
		"unix:path",
		"unix:/path",
		"unix:///path",
		"unix://also/path",
		"unix:///also/path",
	} {
		c := &Check{milterUrl: endpoint}

		err := c.Init(&config.Map{})
		if err != nil {
			t.Errorf("Unexpected failure for %s: %v", endpoint, err)
			return
		}
	}
}

func TestRejectInvalidEndpoints(t *testing.T) {
	for _, endpoint := range []string{
		"tls://0.0.0.0:10025",
		"tls:0.0.0.0:10025",
	} {
		c := &Check{milterUrl: endpoint}
		err := c.Init(&config.Map{})
		if err == nil {
			t.Errorf("Accepted invalid endpoint: %s", endpoint)
			return
		}
	}
}
