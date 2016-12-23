package maddy

import (
	"github.com/mholt/caddy/caddyfile"
	imapserver "github.com/emersion/go-imap/server"
)

func newIMAPServer(tokens map[string][]caddyfile.Token) server {
	return imapserver.New(nil)
}
