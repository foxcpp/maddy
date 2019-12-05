// Package dns defines interfaces used by maddy modules to perform DNS
// lookups.
//
// Currently, there is only Resolver interface which is implemented
// by net.DefaultResolver. In the future, DNSSEC-enabled stub resolver
// implementation will be added here.
package dns

import (
	"context"
	"net"
	"strings"
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

// LookupAddr is a convenience wrapper for Resolver.LookupAddr.
//
// It returns the first name with trailing dot stripped.
func LookupAddr(ctx context.Context, r Resolver, ip net.IP) (string, error) {
	names, err := r.LookupAddr(ctx, ip.String())
	if err != nil || len(names) == 0 {
		return "", err
	}
	return strings.TrimRight(names[0], "."), nil
}
