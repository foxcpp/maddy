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
	if e.Scheme == "unix" {
		return "unix://" + e.Path
	}

	if e.Host == "" && e.Port == "" {
		return ""
	}
	scheme := e.Scheme
	if scheme == "" {
		if e.Port == "993" {
			scheme = "imaps"
		} else {
			scheme = "imap"
		}
	}
	s := scheme
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
	} else {
		return "tcp"
	}
}

func (e Endpoint) Address() string {
	if e.Scheme == "unix" {
		return e.Path
	} else {
		return net.JoinHostPort(e.Host, e.Port)
	}
}

func (e Endpoint) IsTLS() bool {
	return e.Scheme == "tls"
}

// ParseEndpoint parses an endpoint string into a structured format with separate
// scheme, host, port, and path portions, as well as the original input string.
func ParseEndpoint(str string) (Endpoint, error) {
	input := str

	// Split input into components (prepend with // to assert host by default)
	if !strings.Contains(str, "//") && !strings.HasPrefix(str, "/") {
		str = "//" + str
	}
	u, err := url.Parse(str)
	if err != nil {
		return Endpoint{}, err
	}

	switch u.Scheme {
	case "tcp", "tls":
		// ALL GREEN
	case "unix":
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
		return Endpoint{}, fmt.Errorf("unsupported scheme: %s", input)
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
