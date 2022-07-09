//go:build windows || plan9
// +build windows plan9

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

package maddy

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/foxcpp/maddy/framework/log"
)

func handleSignals() os.Signal {
	sig := make(chan os.Signal, 5)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT)

	s := <-sig
	go func() {
		s := handleSignals()
		log.Printf("forced shutdown due to signal (%v)!", s)
		os.Exit(1)
	}()

	log.Printf("signal received (%v), next signal will force immediate shutdown.", s)
	return s
}
