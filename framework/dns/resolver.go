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

// Package dns defines interfaces used by maddy modules to perform DNS
// lookups.
//
// Currently, there is only Resolver interface which is implemented
// by dns.DefaultResolver(). In the future, DNSSEC-enabled stub resolver
// implementation will be added here.
package dns

import (
	"context"
	"net"
	"strings"
)

// Resolver is an interface that describes DNS-related methods used by maddy.
//
// It is implemented by dns.DefaultResolver(). Methods behave the same way.
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

func DefaultResolver() Resolver {
	if overrideServ != "" && overrideServ != "system-default" {
		override(overrideServ)
	}

	return net.DefaultResolver
}
