package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Endpoint represents a site address. It contains
// the original input value, and the component
// parts of an address. The component parts may be
// updated to the correct values as setup proceeds,
// but the original value should never be changed.
type Endpoint struct {
	Original, Scheme, Host, Port, Path string
}

// String returns a human-friendly print of the address.
func (e Endpoint) String() string {
	if e.Scheme == "lmtp+unix" {
		return "lmtp+unix://" + e.Path
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

func (e Endpoint) Protocol() string {
	switch e.Scheme {
	case "imap", "imaps":
		return "imap"
	case "smtp", "smtps":
		return "smtp"
	case "lmtp+unix":
		return "lmtp"
	default:
		return ""
	}
}

func (e Endpoint) Network() string {
	if e.Scheme == "lmtp+unix" {
		return "unix"
	} else {
		return "tcp"
	}
}

func (e Endpoint) Address() string {
	if e.Scheme == "lmtp+unix" {
		return e.Path
	} else {
		return net.JoinHostPort(e.Host, e.Port)
	}
}

func (e Endpoint) IsTLS() bool {
	return e.Scheme == "imaps" || e.Scheme == "smtps"
}

// StandardizeEndpoint parses an endpoint string into a structured format with separate
// scheme, host, port, and path portions, as well as the original input string.
func StandardizeEndpoint(str string) (Endpoint, error) {
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
	case "imap", "imaps", "smtp", "smtps":
		// ALL GREEN
	case "lmtp+unix":
		return Endpoint{Original: input, Scheme: u.Scheme, Path: u.Path}, err
	default:
		return Endpoint{}, fmt.Errorf("[%s] unsupported scheme", input)
	}

	// separate host and port
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		host, port, err = net.SplitHostPort(u.Host + ":")
		if err != nil {
			host = u.Host
		}
	}

	// see if we can set port based off scheme
	if port == "" {
		switch u.Scheme {
		case "imap":
			port = "143"
		case "imaps":
			port = "993"
		case "smtp":
			port = "25"
		case "smtps":
			port = "465"
		}
	}

	// repeated or conflicting scheme is confusing, so error
	if u.Scheme != "" && (port == "imap" || port == "imaps" || port == "smtp" || port == "smtps") {
		return Endpoint{}, fmt.Errorf("[%s] scheme specified twice in address", input)
	}

	// error if scheme and port combination violate convention
	if (u.Scheme == "imap" && port == "993") || (u.Scheme == "imaps" && port == "143") || (u.Scheme == "smtp" && port == "465") || (u.Scheme == "smtps" && port == "25") {
		return Endpoint{}, fmt.Errorf("[%s] scheme and port violate convention", input)
	}

	// standardize http and https ports to their respective port numbers
	// TODO

	return Endpoint{Original: input, Scheme: u.Scheme, Host: host, Port: port, Path: u.Path}, err
}
