package netresource

import (
	"fmt"
	"net"
	"net/netip"

	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/resource"
)

type ListenerTracker struct {
	logger *log.Logger
	tcp    *resource.Tracker[*net.TCPListener]
	unix   *resource.Tracker[*net.UnixListener]
}

func (lt *ListenerTracker) Get(network, addr string) (net.Listener, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
		l, err := lt.tcp.GetOpen(addr, func() (*net.TCPListener, error) {
			addrPort, err := netip.ParseAddrPort(addr)
			if err != nil {
				return nil, err
			}
			lt.logger.DebugMsg("new listener", "network", network, "address", addr)
			return net.ListenTCP(network, net.TCPAddrFromAddrPort(addrPort))
		})
		if err != nil {
			return nil, err
		}

		// We return duplicated listener so when listener is closed by user endpoint
		// the tracked resource remains available and listening on the port doesn't
		// actually stop.
		l2, err := dupTCPListener(l)
		if err != nil {
			return nil, err
		}
		return l2, nil
	case "unix":
		l, err := lt.unix.GetOpen(addr, func() (*net.UnixListener, error) {
			addr, err := net.ResolveUnixAddr(network, addr)
			if err != nil {
				return nil, err
			}
			lt.logger.DebugMsg("new listener", "network", network, "address", addr)
			return net.ListenUnix(network, addr)
		})
		if err != nil {
			return nil, err
		}

		l2, err := dupUnixListener(l)
		if err != nil {
			return nil, err
		}
		return l2, nil
	default:
		return nil, fmt.Errorf("unsupported network type: %s", network)
	}
}

func (lt *ListenerTracker) ResetUsage() {
	lt.tcp.MarkAllUnused()
	lt.unix.MarkAllUnused()
}

func (lt *ListenerTracker) CloseUnused() error {
	lt.tcp.CloseUnused(func(key string) bool {
		return false
	})
	lt.unix.CloseUnused(func(key string) bool {
		return false
	})
	return nil
}

func (lt *ListenerTracker) Close() error {
	lt.tcp.Close()
	lt.unix.Close()
	return nil
}

func NewListenerTracker(log *log.Logger) *ListenerTracker {
	return &ListenerTracker{
		logger: log,
		tcp:    resource.NewTracker[*net.TCPListener](resource.NewSingleton[*net.TCPListener]()),
		unix:   resource.NewTracker[*net.UnixListener](resource.NewSingleton[*net.UnixListener]()),
	}
}
