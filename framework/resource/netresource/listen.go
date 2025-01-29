package netresource

import (
	"fmt"
	"net"
	"strconv"

	"github.com/foxcpp/maddy/framework/log"
)

var (
	tracker = NewListenerTracker(log.DefaultLogger.Sublogger("netresource"))
)

func CloseUnusedListeners() error {
	return tracker.Close()
}

func ResetListenersUsage() {
	tracker.ResetUsage()
}

func Listen(network, addr string) (net.Listener, error) {
	switch network {
	case "fd":
		fd, err := strconv.ParseUint(addr, 10, strconv.IntSize)
		if err != nil {
			return nil, fmt.Errorf("invalid FD number: %v", addr)
		}
		return ListenFD(uint(fd))
	case "fdname":
		return ListenFDName(addr)
	case "tcp", "tcp4", "tcp6", "unix":
		return tracker.Get(network, addr)
	default:
		return nil, fmt.Errorf("unsupported network: %v", network)
	}
}
