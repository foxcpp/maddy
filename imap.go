package maddy

import (
	"github.com/mholt/caddy/caddyfile"
	imapbackend "github.com/emersion/go-imap/backend"
	imapproxy "github.com/emersion/go-imap-proxy"
	imapserver "github.com/emersion/go-imap/server"
)

func newIMAPServer(tokens map[string][]caddyfile.Token) server {
	var be imapbackend.Backend
	if tokens, ok := tokens["proxy"]; ok {
		be = newIMAPProxy(caddyfile.NewDispenserTokens("", tokens))
	}

	return imapserver.New(be)
}

func newIMAPProxy(d caddyfile.Dispenser) imapbackend.Backend {
	for d.Next() {
		args := d.RemainingArgs()

		if len(args) == 1 {
			// TODO: parse args[0] as an Address
			return imapproxy.NewTLS(args[0], nil)
		}
	}

	return nil
}
