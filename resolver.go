package maddy

import (
	"context"
	"net"
)

// Resolver is an interface that describes DNS-related methods used by maddy.
//
// It is implemented by net.DefaultResolver. Methods behave the same way.
type Resolver interface {
	LookupAddr(ctx context.Context, addr string) (names []string, err error)
	LookupHost(ctx context.Context, host string) (addrs []string, err error)
	LookupMX(ctx context.Context, name string) ([]*net.MX, error)
	LookupTXT(ctx context.Context, name string) ([]string, error)
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}
