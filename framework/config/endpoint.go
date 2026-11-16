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
	"strconv"
	"strings"
)

// Endpoint represents a site address. It contains the original input value,
// and the component parts of an address. The component parts may be updated to
// the correct values as setup proceeds, but the original value should never be
// changed.
type Endpoint struct {
	Original, Scheme, SchemeModifier, Host, Port, Path string
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
	if e.Scheme == "" {
		return "tcp"
	}
	return e.Scheme
}

func (e Endpoint) Address() string {
	if e.Scheme == "unix" || e.IsFD() {
		return e.Path
	}
	return net.JoinHostPort(e.Host, e.Port)
}

func (e Endpoint) IsFD() bool {
	return e.Scheme == "fd" || e.Scheme == "fdname"
}

func (e Endpoint) IsTLS() bool {
	return e.SchemeModifier == "tls"
}

func parseFDEndpoint(original string, u *url.URL, scheme, schemeModifier string) (Endpoint, error) {
	// scheme:OPAQUE URL syntax
	if u.Host == "" && u.Opaque != "" {
		u.Host = u.Opaque
	}

	switch schemeModifier {
	case "tls", "":
		// ok
	default:
		return Endpoint{}, fmt.Errorf("unsupported scheme modifier: %s (%s, %+v)", original, schemeModifier, u)
	}

	switch scheme {
	case "fd":
		_, err := strconv.Atoi(u.Host)
		if err != nil {
			return Endpoint{}, fmt.Errorf("failed to parse fd number: %s", err)
		}
	case "fdname":
		// arbitrary path string
	default:
		return Endpoint{}, fmt.Errorf("unsupported scheme: %s (%+v)", original, u)
	}

	return Endpoint{
		Original:       original,
		Scheme:         scheme,
		SchemeModifier: schemeModifier,
		Path:           u.Host,
	}, nil
}

func parseUnixEndpoint(original string, u *url.URL, scheme, schemeModifier string) (Endpoint, error) {
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

	switch schemeModifier {
	case "tls", "":
		// ok
	default:
		return Endpoint{}, fmt.Errorf("unsupported scheme modifier: %s (%s, %+v)", original, schemeModifier, u)
	}

	return Endpoint{
		Original:       original,
		Scheme:         scheme,
		SchemeModifier: schemeModifier,
		Path:           actualPath,
	}, nil
}

func parseTCPEndpoint(original string, u *url.URL, scheme, schemeModifier string) (Endpoint, error) {
	// scheme:OPAQUE URL syntax
	if u.Host == "" && u.Opaque != "" {
		u.Host = u.Opaque
	}

	switch schemeModifier {
	case "tls", "":
	default:
		return Endpoint{}, fmt.Errorf("unsupported scheme modifier: %s (%s, %+v)", original, schemeModifier, u)
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

	return Endpoint{
		Original:       original,
		Scheme:         scheme,
		SchemeModifier: schemeModifier,
		Host:           host,
		Port:           port,
		Path:           u.Path,
	}, err
}

// ParseEndpoint parses an endpoint string into a structured format with separate
// scheme, host, port, and path portions, as well as the original input string.
func ParseEndpoint(str string) (Endpoint, error) {
	input := str

	u, err := url.Parse(str)
	if err != nil {
		return Endpoint{}, err
	}

	scheme := u.Scheme
	schemeModifier := ""

	if strings.Contains(scheme, "+") {
		parts := strings.SplitN(scheme, "+", 2)

		scheme = parts[0]
		schemeModifier = parts[1]
	}

	if scheme == "tls" {
		scheme = "tcp"
		schemeModifier = "tls"
	}

	switch scheme {
	case "fd", "fdname":
		return parseFDEndpoint(input, u, scheme, schemeModifier)
	case "tcp", "tcp4", "tcp6":
		return parseTCPEndpoint(input, u, scheme, schemeModifier)
	case "unix":
		return parseUnixEndpoint(input, u, scheme, schemeModifier)
	default:
		return Endpoint{}, fmt.Errorf("unsupported scheme: %s (%+v)", input, u)
	}
}
