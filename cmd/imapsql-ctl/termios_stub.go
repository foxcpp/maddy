//+build !linux

package main

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
