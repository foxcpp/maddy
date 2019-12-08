package dns

import (
	"context"
	"net"
	"time"
)

var (
	overrideServ string
)

// override globally overrides the used DNS server address with one provided.
// This function is meant only for testing. It should be called before any modules are
// initialized to have full effect.
//
// The server argument is in form of "IP:PORT". It is expected that the server
// will be available both using TCP and UDP on the same port.
func override(server string) {
	net.DefaultResolver.Dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		dialer := net.Dialer{
			// This is localhost, it is either running or not. Fail quickly if
			// we can't connect.
			Timeout: 1 * time.Second,
		}

		switch network {
		case "udp", "udp4", "udp6":
			return dialer.DialContext(ctx, "udp4", server)
		case "tcp", "tcp4", "tcp6":
			return dialer.DialContext(ctx, "tcp4", server)
		default:
			panic("OverrideDNS.Dial: unknown network")
		}
	}
}
