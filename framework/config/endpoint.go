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

package config

import (
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strings"
)

// Endpoint represents a site address. It contains the original input value,
// and the component parts of an address. The component parts may be updated to
// the correct values as setup proceeds, but the original value should never be
// changed.
type Endpoint struct {
	Original, Scheme, Host, Port, Path string
}

// String returns a human-friendly print of the address.
func (e Endpoint) String() string {
	if e.Original != "" {
		return e.Original
	}

	if e.Scheme == "unix" {
		return "unix://" + e.Path
	}

	if e.Host == "" && e.Port == "" {
		return ""
	}
	s := e.Scheme
	if s != "" {
		s += "://"
	}

	host := e.Host
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	s += host

	if e.Port != "" {
		s += ":" + e.Port
	}
	if e.Path != "" {
		s += e.Path
	}
	return s
}

func (e Endpoint) Network() string {
	if e.Scheme == "unix" {
		return "unix"
	}
	return "tcp"
}

func (e Endpoint) Address() string {
	if e.Scheme == "unix" {
		return e.Path
	}
	return net.JoinHostPort(e.Host, e.Port)
}

func (e Endpoint) IsTLS() bool {
	return e.Scheme == "tls"
}

// ParseEndpoint parses an endpoint string into a structured format with separate
// scheme, host, port, and path portions, as well as the original input string.
func ParseEndpoint(str string) (Endpoint, error) {
	input := str

	u, err := url.Parse(str)
	if err != nil {
		return Endpoint{}, err
	}

	switch u.Scheme {
	case "tcp", "tls":
		// ALL GREEN

		// scheme:OPAQUE URL syntax
		if u.Host == "" && u.Opaque != "" {
			u.Host = u.Opaque
		}
	case "unix":
		// scheme:OPAQUE URL syntax
		if u.Path == "" && u.Opaque != "" {
			u.Path = u.Opaque
		}

		var actualPath string
		if u.Host != "" {
			actualPath += u.Host
		}
		if u.Path != "" {
			actualPath += u.Path
		}

		if !filepath.IsAbs(actualPath) {
			actualPath = filepath.Join(RuntimeDirectory, actualPath)
		}

		return Endpoint{Original: input, Scheme: u.Scheme, Path: actualPath}, err
	default:
		return Endpoint{}, fmt.Errorf("unsupported scheme: %s (%+v)", input, u)
	}

	// separate host and port
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		host, port, err = net.SplitHostPort(u.Host + ":")
		if err != nil {
			host = u.Host
		}
	}
	if port == "" {
		return Endpoint{}, fmt.Errorf("port is required")
	}

	return Endpoint{Original: input, Scheme: u.Scheme, Host: host, Port: port, Path: u.Path}, err
}
