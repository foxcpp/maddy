package maddy

import (
	"github.com/mholt/caddy/caddyfile"
	"github.com/emersion/go-smtp"
	smtpproxy "github.com/emersion/go-smtp-proxy"
)

func newSMTPServer(tokens map[string][]caddyfile.Token) server {
	var be smtp.Backend
	if tokens, ok := tokens["proxy"]; ok {
		be = newSMTPProxy(caddyfile.NewDispenserTokens("", tokens))
	}

	return smtp.NewServer(be)
}

func newSMTPProxy(d caddyfile.Dispenser) smtp.Backend {
	for d.Next() {
		args := d.RemainingArgs()

		if len(args) == 1 {
			// TODO: parse args[0] as an Address
			return smtpproxy.NewTLS(args[0], nil)
		}
	}

	return nil
}
