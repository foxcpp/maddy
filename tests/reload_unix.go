//go:build unix

/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2026 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

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

package tests

import (
	"syscall"
	"time"
)

func (t *T) reloadConfig() {
	err := t.servProc.Process.Signal(syscall.SIGUSR2)
	if err != nil {
		t.Fatal("Failed to send SIGUSR2:", err)
	}

	t.Log("waiting for server to reload...")

	select {
	case <-t.reloadedChan:
	case <-time.After(5 * time.Second):
		t.killServer()
		t.Fatal("Server reload is taking too long, killed")
	}
}
