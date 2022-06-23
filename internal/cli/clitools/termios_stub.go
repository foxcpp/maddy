//go:build !linux
// +build !linux

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

package clitools

import (
	"errors"
	"os"
)

type Termios struct {
	Iflag  uint32
	Oflag  uint32
	Cflag  uint32
	Lflag  uint32
	Cc     [20]byte
	Ispeed uint32
	Ospeed uint32
}

func TurnOnRawIO(tty *os.File) (orig Termios, err error) {
	return Termios{}, errors.New("not implemented")
}

func TcSetAttr(fd uintptr, termios *Termios) error {
	return errors.New("not implemented")
}

func TcGetAttr(fd uintptr) (*Termios, error) {
	return nil, errors.New("not implemented")
}
