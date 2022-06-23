//go:build linux
// +build linux

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

// Copied from github.com/foxcpp/ttyprompt
// Commit 087a574, terminal/termios.go

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
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

/*
TurnOnRawIO sets flags suitable for raw I/O (no echo, per-character input, etc)
and returns original flags.
*/
func TurnOnRawIO(tty *os.File) (orig Termios, err error) {
	termios, err := TcGetAttr(tty.Fd())
	if err != nil {
		return Termios{}, errors.New("TurnOnRawIO: failed to get flags: " + err.Error())
	}
	termiosOrig := *termios

	termios.Lflag &^= syscall.ECHO
	termios.Lflag &^= syscall.ICANON
	termios.Iflag &^= syscall.IXON
	termios.Lflag &^= syscall.ISIG
	termios.Iflag |= syscall.IUTF8
	err = TcSetAttr(tty.Fd(), termios)
	if err != nil {
		return Termios{}, errors.New("TurnOnRawIO: flags to set flags: " + err.Error())
	}
	return termiosOrig, nil
}

func TcSetAttr(fd uintptr, termios *Termios) error {
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TCSETS, uintptr(unsafe.Pointer(termios)))
	if err != 0 {
		return err
	}
	return nil
}

func TcGetAttr(fd uintptr) (*Termios, error) {
	termios := &Termios{}
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TCGETS, uintptr(unsafe.Pointer(termios)))
	if err != 0 {
		return nil, err
	}
	return termios, nil
}
