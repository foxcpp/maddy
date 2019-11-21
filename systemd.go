//+build linux

package maddy

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"syscall"

	"github.com/foxcpp/maddy/log"
)

type SDStatus string

const (
	SDReady    = "READY=1"
	SDStopping = "STOPPING=1"
)

var (
	ErrNoNotifySock = errors.New("no systemd socket")
)

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
	if err := sConn.Control(func(fd uintptr) {
		syscall.SetsockoptByte(int(fd), syscall.SOL_SOCKET, syscall.SO_PASSCRED, 1)
	}); err != nil {
		return err
	}
	return nil
}

func systemdStatus(status SDStatus, desc string) {
	sock, err := sdNotifySock()
	if err != nil {
		if err != ErrNoNotifySock {
			log.Println("systemd: failed to acquire notify socket:", err)
		}
		return
	}
	defer sock.Close()

	if err := setScmPassCred(sock); err != nil {
		log.Println("failed to set SCM_PASSCRED on the socket for communication with systemd:", err)
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
		if err != ErrNoNotifySock {
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
