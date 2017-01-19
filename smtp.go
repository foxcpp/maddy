package maddy

import (
	"github.com/mholt/caddy/caddyfile"
	"github.com/emersion/go-smtp"
	smtpproxy "github.com/emersion/go-smtp-proxy"
	smtppgp "github.com/emersion/go-pgpmail/smtp"
	pgplocal "github.com/emersion/go-pgpmail/local"
	pgppubkey "github.com/emersion/go-pgp-pubkey"
	pgppubkeylocal "github.com/emersion/go-pgp-pubkey/local"
	"golang.org/x/crypto/openpgp"
)

func newSMTPServer(tokens map[string][]caddyfile.Token) (server, error) {
	var be smtp.Backend
	if tokens, ok := tokens["proxy"]; ok {
		var err error
		be, err = newSMTPProxy(caddyfile.NewDispenserTokens("", tokens))
		if err != nil {
			return nil, err
		}
	}

	s := smtp.NewServer(be)
	return s, nil
}

func newSMTPProxy(d caddyfile.Dispenser) (smtp.Backend, error) {
	for d.Next() {
		args := d.RemainingArgs()

		if len(args) == 1 {
			// TODO: parse args[0] as an Address
			return smtpproxy.NewTLS(args[0], nil), nil
		}
	}

	return nil, nil
}

type keyRing struct {
	pgppubkey.Source
}

func (kr *keyRing) Unlock(username, password string) (openpgp.EntityList, error) {
	return pgplocal.Unlock(username, password)
}

func newSMTPPGP(d caddyfile.Dispenser, be smtp.Backend) (smtp.Backend, error) {
	kr := &keyRing{pgppubkeylocal.New()}
	pgpbe := smtppgp.New(be, kr)
	return pgpbe, nil
}
