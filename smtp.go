package maddy

import (
	"github.com/mholt/caddy/caddyfile"
	"github.com/emersion/go-smtp"
)

func newSMTPServer(tokens map[string][]caddyfile.Token) server {
	return smtp.NewServer(nil)
}
