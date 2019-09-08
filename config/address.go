package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Address represents a site address. It contains
// the original input value, and the component
// parts of an address. The component parts may be
// updated to the correct values as setup proceeds,
// but the original value should never be changed.
type Address struct {
	Original, Scheme, Host, Port, Path string
}

// String returns a human-friendly print of the address.
func (a Address) String() string {
	if a.Scheme == "lmtp+unix" {
		return "lmtp+unix://" + a.Path
	}

	if a.Host == "" && a.Port == "" {
		return ""
	}
	scheme := a.Scheme
	if scheme == "" {
		if a.Port == "993" {
			scheme = "imaps"
		} else {
			scheme = "imap"
		}
	}
	s := scheme
	if s != "" {
		s += "://"
	}

	host := a.Host
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	s += host

	if a.Port != "" {
		s += ":" + a.Port
	}
	if a.Path != "" {
		s += a.Path
	}
	return s
}

func (a Address) Protocol() string {
	switch a.Scheme {
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

func (a Address) Network() string {
	if a.Scheme == "lmtp+unix" {
		return "unix"
	} else {
		return "tcp"
	}
}

func (a Address) Address() string {
	if a.Scheme == "lmtp+unix" {
		return a.Path
	} else {
		return net.JoinHostPort(a.Host, a.Port)
	}
}

func (a Address) IsTLS() bool {
	return a.Scheme == "imaps" || a.Scheme == "smtps"
}

// StandardizeAddress parses an address string into a structured format with separate
// scheme, host, port, and path portions, as well as the original input string.
func StandardizeAddress(str string) (Address, error) {
	input := str

	// Split input into components (prepend with // to assert host by default)
	if !strings.Contains(str, "//") && !strings.HasPrefix(str, "/") {
		str = "//" + str
	}
	u, err := url.Parse(str)
	if err != nil {
		return Address{}, err
	}

	switch u.Scheme {
	case "imap", "imaps", "smtp", "smtps":
		// ALL GREEN
	case "lmtp+unix":
		return Address{Original: input, Scheme: u.Scheme, Path: u.Path}, err
	default:
		return Address{}, fmt.Errorf("[%s] unsupported scheme", input)
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
		return Address{}, fmt.Errorf("[%s] scheme specified twice in address", input)
	}

	// error if scheme and port combination violate convention
	if (u.Scheme == "imap" && port == "993") || (u.Scheme == "imaps" && port == "143") || (u.Scheme == "smtp" && port == "465") || (u.Scheme == "smtps" && port == "25") {
		return Address{}, fmt.Errorf("[%s] scheme and port violate convention", input)
	}

	// standardize http and https ports to their respective port numbers
	// TODO

	return Address{Original: input, Scheme: u.Scheme, Host: host, Port: port, Path: u.Path}, err
}
