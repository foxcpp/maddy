package maddy

import (
	"io"
	"log"
	"os"

	"github.com/mholt/caddy/caddyfile"
	imapbackend "github.com/emersion/go-imap/backend"
	imapserver "github.com/emersion/go-imap/server"
	imapproxy "github.com/emersion/go-imap-proxy"
	imapcompress "github.com/emersion/go-imap-compress"
	imappgp "github.com/emersion/go-pgpmail/imap"
	pgplocal "github.com/emersion/go-pgpmail/local"
)

func newIMAPServer(tokens map[string][]caddyfile.Token) (server, error) {
	var be imapbackend.Backend
	if tokens, ok := tokens["proxy"]; ok {
		var err error
		be, err = newIMAPProxy(caddyfile.NewDispenserTokens("", tokens))
		if err != nil {
			return nil, err
		}
	}

	if tokens, ok := tokens["pgp"]; ok {
		var err error
		be, err = newIMAPPGP(caddyfile.NewDispenserTokens("", tokens), be)
		if err != nil {
			return nil, err
		}
	}

	s := imapserver.New(be)

	if tokens, ok := tokens["errors"]; ok {
		err := setIMAPErrors(caddyfile.NewDispenserTokens("", tokens), s)
		if err != nil {
			return nil, err
		}
	}
	if _, ok := tokens["compress"]; ok {
		s.Enable(imapcompress.NewExtension())
	}

	return s, nil
}

func newIMAPProxy(d caddyfile.Dispenser) (imapbackend.Backend, error) {
	for d.Next() {
		args := d.RemainingArgs()

		if len(args) == 1 {
			addr, err := standardizeAddress(args[0])
			if err != nil {
				return nil, err
			}

			target := addr.Host + ":" + addr.Port
			if addr.IsTLS() {
				return imapproxy.NewTLS(target, nil), nil
			} else {
				return imapproxy.New(target), nil
			}
		}
	}

	return nil, nil
}

func newIMAPPGP(d caddyfile.Dispenser, be imapbackend.Backend) (imapbackend.Backend, error) {
	pgpbe := imappgp.New(be, pgplocal.Unlock)
	return pgpbe, nil
}

func setIMAPErrors(d caddyfile.Dispenser, s *imapserver.Server) error {
	for d.Next() {
		args := d.RemainingArgs()

		if len(args) == 1 {
			output := args[0]
			var w io.Writer
			switch output {
			case "stdout":
				w = os.Stdout
			case "stderr":
				w = os.Stderr
			default:
				f, err := os.OpenFile(output, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
				if err != nil {
					return err
				}
				w = f
			}

			s.ErrorLog = log.New(w, "imap", log.LstdFlags)
		}
	}

	return nil
}
