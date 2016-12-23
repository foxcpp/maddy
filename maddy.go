package maddy

import (
	"fmt"
	"log"
	"net"

	"github.com/mholt/caddy/caddyfile"
	imapserver "github.com/emersion/go-imap/server"
	"github.com/emersion/go-smtp"
)

var Directives = []string{
	"bind",
	"tls",

	"log",
	"errors",
	"compress",
	"proxy",
}

func Start(blocks []caddyfile.ServerBlock) error {
	for _, block := range blocks {
		var proto string
		var adresses []Address
		for _, k := range block.Keys {
			addr, err := standardizeAddress(k)
			if err != nil {
				return fmt.Errorf("Cannot parse block key %q: %v", k, err)
			}

			p := addr.Protocol()
			if proto == "" {
				proto = p
			} else if proto != p {
				return fmt.Errorf("Block contains incompatible protocols: %s and %s", proto, p)
			}

			adresses = append(adresses, addr)
		}

		// TODO: parse directives

		switch proto {
		case "imap":
			s := imapserver.New(nil) // TODO

			for _, addr := range adresses {
				l, err := net.Listen("tcp", fmt.Sprintf("%s:%s", addr.Host, addr.Port))
				if err != nil {
					return fmt.Errorf("Cannot listen: %v", err)
				}

				log.Println("IMAP server listening on", l.Addr().String())
				go s.Serve(l)
			}
		case "smtp":
			s := smtp.NewServer(nil) // TODO

			for _, addr := range adresses {
				l, err := net.Listen("tcp", fmt.Sprintf("%s:%s", addr.Host, addr.Port))
				if err != nil {
					return fmt.Errorf("Cannot listen: %v", err)
				}

				log.Println("SMTP server listening on", l.Addr().String())
				go s.Serve(l)
			}
		default:
			return fmt.Errorf("Unsupported protocol %q", proto)
		}
	}

	select {}
}
