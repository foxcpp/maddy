package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/mholt/caddy/caddyfile"
	imapserver "github.com/emersion/go-imap/server"
	"github.com/emersion/go-smtp"
)

var directives = []string{
	"bind",
	"tls",

	"log",
	"errors",
	"compress",
	"proxy",
}

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
	s += a.Host
	if a.Port != "" &&
		((scheme == "imaps" && a.Port != "993") ||
			(scheme == "imap" && a.Port != "143") ||
			(scheme == "smtps" && a.Port != "465") ||
			(scheme == "smtp" && a.Port != "25")) {
		s += ":" + a.Port
	}
	if a.Path != "" {
		s += a.Path
	}
	return s
}

// VHost returns a sensible concatenation of Host:Port/Path from a.
// It's basically the a.Original but without the scheme.
func (a Address) VHost() string {
	if idx := strings.Index(a.Original, "://"); idx > -1 {
		return a.Original[idx+3:]
	}
	return a.Original
}

// standardizeAddress parses an address string into a structured format with separate
// scheme, host, port, and path portions, as well as the original input string.
func standardizeAddress(str string) (Address, error) {
	input := str

	// Split input into components (prepend with // to assert host by default)
	if !strings.Contains(str, "//") && !strings.HasPrefix(str, "/") {
		str = "//" + str
	}
	u, err := url.Parse(str)
	if err != nil {
		return Address{}, err
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
	if (u.Scheme == "imap" && port == "993") || (u.Scheme == "imaps" && port == "143") ||
		(u.Scheme == "smtp" && port == "465") || (u.Scheme == "smtps" && port == "25") {
		return Address{}, fmt.Errorf("[%s] scheme and port violate convention", input)
	}

	// standardize http and https ports to their respective port numbers
	// TODO

	return Address{Original: input, Scheme: u.Scheme, Host: host, Port: port, Path: u.Path}, err
}

func main() {
	var configpath string
	flag.StringVar(&configpath, "config", "Maddyfile", "path to Maddyfile")
	flag.Parse()

	f, err := os.Open(configpath)
	if err != nil {
		log.Fatalf("Cannot open %q: %v", configpath, err)
	}
	defer f.Close()

	blocks, err := caddyfile.Parse(configpath, f, directives)
	if err != nil {
		log.Fatalf("Cannot parse %q: %v", configpath, err)
	}

	for _, block := range blocks {
		var scheme string
		var adresses []Address
		for _, k := range block.Keys {
			addr, err := standardizeAddress(k)
			if err != nil {
				log.Fatalf("Cannot parse block key %q in %q: %v", k, configpath, err)
			}

			if scheme == "" {
				scheme = addr.Scheme
			}

			adresses = append(adresses, addr)
		}

		// TODO: parse directives

		switch scheme {
		case "imap", "imaps":
			s := imapserver.New(nil) // TODO

			for _, addr := range adresses {
				l, err := net.Listen("tcp", fmt.Sprintf("%s:%s", addr.Host, addr.Port))
				if err != nil {
					log.Fatalf("Cannot listen: %v", err)
				}

				log.Println("IMAP server listening on", l.Addr().String())
				go s.Serve(l)
			}
		case "smtp", "smtps":
			s := smtp.NewServer(nil) // TODO

			for _, addr := range adresses {
				l, err := net.Listen("tcp", fmt.Sprintf("%s:%s", addr.Host, addr.Port))
				if err != nil {
					log.Fatalf("Cannot listen: %v", err)
				}

				log.Println("SMTP server listening on", l.Addr().String())
				go s.Serve(l)
			}
		default:
			log.Fatalf("Unsupported scheme %q in %q", scheme, configpath)
		}
	}

	select {}
}
