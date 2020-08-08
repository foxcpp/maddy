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
	"time"
)

type Output interface {
	Write(stamp time.Time, debug bool, msg string)
	Close() error
}

type multiOut struct {
	outs []Output
}

func (m multiOut) Write(stamp time.Time, debug bool, msg string) {
	for _, out := range m.outs {
		out.Write(stamp, debug, msg)
	}
}

func (m multiOut) Close() error {
	for _, out := range m.outs {
		if err := out.Close(); err != nil {
			return err
		}
	}
	return nil
}

func MultiOutput(outputs ...Output) Output {
	return multiOut{outputs}
}

type funcOut struct {
	out   func(time.Time, bool, string)
	close func() error
}

func (f funcOut) Write(stamp time.Time, debug bool, msg string) {
	f.out(stamp, debug, msg)
}

func (f funcOut) Close() error {
	return f.close()
}

func FuncOutput(f func(time.Time, bool, string), close func() error) Output {
	return funcOut{f, close}
}

type NopOutput struct{}

func (NopOutput) Write(time.Time, bool, string) {}

func (NopOutput) Close() error { return nil }
