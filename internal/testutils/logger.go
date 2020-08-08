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

package testutils

import (
	"flag"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/foxcpp/maddy/framework/log"
)

var (
	debugLog  = flag.Bool("test.debuglog", false, "(maddy) Turn on debug log messages")
	directLog = flag.Bool("test.directlog", false, "(maddy) Log to stderr instead of test log")
)

func Logger(t *testing.T, name string) log.Logger {
	if *directLog {
		return log.Logger{
			Out:   log.WriterOutput(os.Stderr, true),
			Name:  name,
			Debug: *debugLog,
		}
	}

	return log.Logger{
		Out: log.FuncOutput(func(_ time.Time, debug bool, str string) {
			t.Helper()
			str = strings.TrimSuffix(str, "\n")
			if debug {
				str = "[debug] " + str
			}
			t.Log(str)
		}, func() error {
			return nil
		}),
		Name:  name,
		Debug: *debugLog,
	}
}
