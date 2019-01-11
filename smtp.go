package maddy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-msgauth"
	pgppubkey "github.com/emersion/go-pgp-pubkey"
	pgppubkeylocal "github.com/emersion/go-pgp-pubkey/local"
	pgplocal "github.com/emersion/go-pgpmail/local"
	smtppgp "github.com/emersion/go-pgpmail/smtp"
	"github.com/emersion/go-smtp"
	smtpproxy "github.com/emersion/go-smtp-proxy"
	"github.com/emersion/go-smtp/backendutil"
	"github.com/mholt/caddy/caddyfile"
	"golang.org/x/crypto/openpgp"
)

func newSMTPServer(tokens map[string][]caddyfile.Token) (server, error) {
	// TODO: disallow both proxy and debug to be specified at the same time
	var be smtp.Backend
	if tokens, ok := tokens["proxy"]; ok {
		var err error
		be, err = newSMTPProxy(caddyfile.NewDispenserTokens("", tokens))
		if err != nil {
			return nil, err
		}
	}

	if tokens, ok := tokens["debug"]; ok {
		var err error
		be, err = newSMTPDebug(caddyfile.NewDispenserTokens("", tokens))
		if err != nil {
			return nil, err
		}
	}

	if tokens, ok := tokens["auth"]; ok {
		var err error
		be, err = newSMTPAuth(caddyfile.NewDispenserTokens("", tokens), be)
		if err != nil {
			return nil, err
		}
	}

	if tokens, ok := tokens["pgp"]; ok {
		var err error
		be, err = newSMTPPGP(caddyfile.NewDispenserTokens("", tokens), be)
		if err != nil {
			return nil, err
		}
	}

	if be == nil {
		return nil, errors.New("missing SMTP upstream")
	}

	s := smtp.NewServer(newVerifier(newRelay(be)))
	return s, nil
}

func newSMTPProxy(d caddyfile.Dispenser) (smtp.Backend, error) {
	for d.Next() {
		args := d.RemainingArgs()

		if len(args) == 1 {
			addr, err := standardizeAddress(args[0])
			if err != nil {
				return nil, err
			}

			if addr.Scheme == "lmtp+unix" {
				return smtpproxy.NewLMTP(addr.Path, "localhost"), nil
			}

			if addr.IsTLS() {
				return smtpproxy.NewTLS(addr.Address(), nil), nil
			} else {
				return smtpproxy.New(addr.Address()), nil
			}
		}
	}

	return nil, nil
}

type debugUser struct{}

func (u debugUser) Send(from string, to []string, r io.Reader) error {
	fmt.Println("MAIL FROM:", from)
	for _, addr := range to {
		fmt.Println("RCPT TO:", addr)
	}
	io.Copy(os.Stdout, r)
	return nil
}

func (u debugUser) Logout() error {
	return nil
}

type debugBackend struct{}

func (be debugBackend) Login(username, password string) (smtp.User, error) {
	return debugUser{}, nil
}

func (be debugBackend) AnonymousLogin() (smtp.User, error) {
	return debugUser{}, nil
}

func newSMTPDebug(d caddyfile.Dispenser) (smtp.Backend, error) {
	return debugBackend{}, nil
}

func newSMTPAuth(d caddyfile.Dispenser, be smtp.Backend) (smtp.Backend, error) {
	return nil, nil // TODO
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

func newVerifier(be smtp.Backend) smtp.Backend {
	identity := "localhost"

	return &backendutil.TransformBackend{
		Backend: be,
		Transform: func(from string, to []string, r io.Reader) (string, []string, io.Reader, error) {
			var b bytes.Buffer
			if _, err := io.Copy(&b, r); err != nil {
				return "", nil, nil, err
			}

			results, err := verifyDKIM(bytes.NewReader(b.Bytes()))
			if err != nil {
				return "", nil, nil, err
			}

			// TODO: strip existing Authentication-Results header fields with our identity

			// TODO: don't print Authentication-Results on a single line
			authRes := "Authentication-Results: " + msgauth.Format(identity, results) + "\r\n"
			r = io.MultiReader(strings.NewReader(authRes), &b)
			return from, to, r, nil
		},
	}
}

func newRelay(be smtp.Backend) smtp.Backend {
	hostname := "localhost" // TODO

	return &backendutil.TransformBackend{
		Backend: be,
		Transform: func(from string, to []string, r io.Reader) (string, []string, io.Reader, error) {
			// TODO: don't print Received on a single line
			received := "Received:"
			// TODO: from
			received += " by " + hostname
			received += " with ESMTP"
			// TODO: for
			received += "; " + time.Now().Format(time.RFC1123Z) + "\r\n"
			// TODO: add comments with TLS information

			r = io.MultiReader(strings.NewReader(received), r)
			return from, to, r, nil
		},
	}
}
