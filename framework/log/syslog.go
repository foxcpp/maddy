//go:build !windows && !plan9
// +build !windows,!plan9

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

package log

import (
	"fmt"
	"log/syslog"
	"os"
	"time"
)

type syslogOut struct {
	w *syslog.Writer
}

func (s syslogOut) Write(stamp time.Time, debug bool, msg string) {
	var err error
	if debug {
		err = s.w.Debug(msg + "\n")
	} else {
		err = s.w.Info(msg + "\n")
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "!!! Failed to send message to syslog daemon: %v\n", err)
	}
}

func (s syslogOut) Close() error {
	return s.w.Close()
}

// SyslogOutput returns a log.Output implementation that will send
// messages to the system syslog daemon.
//
// Regular messages will be written with INFO priority,
// debug messages will be written with DEBUG priority.
//
// Returned log.Output object is goroutine-safe.
func SyslogOutput() (Output, error) {
	w, err := syslog.New(syslog.LOG_MAIL|syslog.LOG_INFO, "maddy")
	return syslogOut{w}, err
}
