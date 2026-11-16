package netresource

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	EnvListenPID    = "LISTEN_PID"
	EnvListenPIDAny = "LISTEN_PID_ANY"
	EnvListenFDName = "LISTEN_FDNAMES"
)

func ListenFD(fd uint) (net.Listener, error) {
	file := os.NewFile(uintptr(fd), strconv.FormatUint(uint64(fd), 10))
	defer func() {
		if err := file.Close(); err != nil {
			panic(err)
		}
	}()
	return net.FileListener(file)
}

func ListenFDName(name string) (net.Listener, error) {
	if os.Getenv(EnvListenPIDAny) != "1" {
		listenPDStr := os.Getenv(EnvListenPID)
		if listenPDStr == "" {
			return nil, errors.New("$LISTEN_PID is not set")
		}
		listenPid, err := strconv.Atoi(listenPDStr)
		if err != nil {
			return nil, errors.New("$LISTEN_PID is not integer")
		}
		if listenPid != os.Getpid() {
			return nil, fmt.Errorf("$LISTEN_PID (%d) is not our PID (%d)", listenPid, os.Getpid())
		}
	}

	names := strings.Split(os.Getenv(EnvListenFDName), ":")
	fd := uintptr(0)
	for i, fdName := range names {
		if fdName == name {
			fd = uintptr(3 + i)
			break
		}
	}

	if fd == 0 {
		return nil, fmt.Errorf("name %s not found in $LISTEN_FDNAMES", name)
	}

	file := os.NewFile(fd, name)
	defer func() {
		if err := file.Close(); err != nil {
			panic(err)
		}
	}()
	return net.FileListener(file)
}
