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

package maddy

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"syscall"

	"github.com/foxcpp/maddy/framework/log"
)

type SDStatus string

const (
	SDReady     = "READY=1"
	SDReloading = "RELOADING=1"
	SDStopping  = "STOPPING=1"
)

var ErrNoNotifySock = errors.New("no systemd socket")

func sdNotifySock() (*net.UnixConn, error) {
	sockAddr := os.Getenv("NOTIFY_SOCKET")
	if sockAddr == "" {
		return nil, ErrNoNotifySock
	}
	if strings.HasPrefix(sockAddr, "@") {
		sockAddr = "\x00" + sockAddr[1:]
	}

	return net.DialUnix("unixgram", nil, &net.UnixAddr{
		Name: sockAddr,
		Net:  "unixgram",
	})
}

func setScmPassCred(sock *net.UnixConn) error {
	sConn, err := sock.SyscallConn()
	if err != nil {
		return err
	}

	var sockoptErr error
	if err := sConn.Control(func(fd uintptr) {
		sockoptErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_PASSCRED, 1)
	}); err != nil {
		return err
	}
	if sockoptErr != nil {
		return sockoptErr
	}
	return nil
}

func systemdStatus(status SDStatus, desc string) {
	sock, err := sdNotifySock()
	if err != nil {
		if !errors.Is(err, ErrNoNotifySock) {
			log.Println("systemd: failed to acquire notify socket:", err)
		}
		return
	}
	defer sock.Close()

	if err := setScmPassCred(sock); err != nil {
		log.Println("systemd: failed to set SCM_PASSCRED on the socket:", err)
	}

	if desc != "" {
		if _, err := io.WriteString(sock, fmt.Sprintf("%s\nSTATUS=%s", status, desc)); err != nil {
			log.Println("systemd: I/O error:", err)
		}
		log.Debugf(`systemd: %s STATUS="%s"`, status, desc)
	} else {
		if _, err := io.WriteString(sock, string(status)); err != nil {
			log.Println("systemd: I/O error:", err)
		}
		log.Debugf(`systemd: %s`, status)
	}
}

func systemdStatusErr(reportedErr error) {
	sock, err := sdNotifySock()
	if err != nil {
		if !errors.Is(err, ErrNoNotifySock) {
			log.Println("systemd: failed to acquire notify socket:", err)
		}
		return
	}
	defer sock.Close()

	if err := setScmPassCred(sock); err != nil {
		log.Println("systemd: failed to set SCM_PASSCRED on the socket:", err)
	}

	var errno syscall.Errno
	if errors.As(reportedErr, &errno) {
		log.Debugf(`systemd: ERRNO=%d STATUS="%v"`, errno, reportedErr)
		if _, err := io.WriteString(sock, fmt.Sprintf("ERRNO=%d\nSTATUS=%v", errno, reportedErr)); err != nil {
			log.Println("systemd: I/O error:", err)
		}
		return
	}

	if _, err := io.WriteString(sock, fmt.Sprintf("STATUS=%v\n", reportedErr)); err != nil {
		log.Println("systemd: I/O error:", err)
	}
	log.Debugf(`systemd: STATUS="%v"`, reportedErr)
}
