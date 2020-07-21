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

package config

var (
	// StateDirectory contains the path to the directory that
	// should be used to store any data that should be
	// preserved between sessions.
	//
	// Value of this variable must not change after initialization
	// in cmd/maddy/main.go.
	StateDirectory string

	// RuntimeDirectory contains the path to the directory that
	// should be used to store any temporary data.
	//
	// It should be preferred over os.TempDir, which is
	// global and world-readable on most systems, while
	// RuntimeDirectory can be dedicated for maddy.
	//
	// Value of this variable must not change after initialization
	// in cmd/maddy/main.go.
	RuntimeDirectory string

	// LibexecDirectory contains the path to the directory
	// where helper binaries should be searched.
	//
	// Value of this variable must not change after initialization
	// in cmd/maddy/main.go.
	LibexecDirectory string
)
