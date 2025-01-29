package netresource

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

func ListenFD(fd uint) (net.Listener, error) {
	file := os.NewFile(uintptr(fd), strconv.FormatUint(uint64(fd), 10))
	defer file.Close()
	return net.FileListener(file)
}

func ListenFDName(name string) (net.Listener, error) {
	listenPDStr := os.Getenv("LISTEN_PID")
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

	names := strings.Split(os.Getenv("LISTEN_FDNAMES"), ":")
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

	file := os.NewFile(3+fd, name)
	defer file.Close()
	return net.FileListener(file)
}
