package maddy

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	imapbackend "github.com/emersion/go-imap/backend"
	imapserver "github.com/emersion/go-imap/server"
	"github.com/emersion/maddy/module"
	"github.com/mholt/caddy/caddyfile"
)

type IMAPEndpoint struct {
	name  string
	serv  *imapserver.Server
	Auth  module.AuthProvider
	Store module.Storage
}

func NewIMAPEndpoint(instName string, cfg map[string][]caddyfile.Token) (module.Module, error) {
	var err error
	endp := new(IMAPEndpoint)
	endp.name = instName

	if tokens, ok := cfg["auth"]; ok {
		endp.Auth, err = authProvider(tokens)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("auth. provider is not set")
	}

	if tokens, ok := cfg["storage"]; ok {
		endp.Store, err = storageBackend(tokens)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("storage backend is not set")
	}

	var tlsConf *tls.Config
	if tokens, ok := cfg["tls"]; ok {
		if err := setTLS(caddyfile.NewDispenserTokens("", tokens), &tlsConf); err != nil {
			return nil, err
		}
	}

	// TODO: Should we special-case endpoint modules for sake of mode caddy-like
	// syntax for multiple listen addresses?
	addr, err := standardizeAddress(instName)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %s", instName)
	}
	addresses := []Address{addr}

	endp.serv = imapserver.New(endp)

	if _, ok := cfg["insecureauth"]; ok {
		log.Printf("imap %v: insecure auth is on!\n", instName)
		endp.serv.AllowInsecureAuth = true
	}
	if _, ok := cfg["iodebug"]; ok {
		log.Printf("imap %v: I/O debugging is on!\n", instName)
		endp.serv.Debug = os.Stderr
	}
	if tokens, ok := cfg["errors"]; ok {
		err := setIMAPErrors(instName, caddyfile.NewDispenserTokens("", tokens), endp.serv)
		if err != nil {
			return nil, err
		}
	}

	// TODO: Enable extensions here.

	for _, addr := range addresses {
		var l net.Listener
		var err error
		l, err = net.Listen(addr.Network(), addr.Address())
		if err != nil {
			return nil, fmt.Errorf("failed to bind on %v: %v", addr, err)
		}

		if addr.IsTLS() {
			l = tls.NewListener(l, tlsConf)
		}

		go func() {
			endp.serv.Serve(l)
		}()
		log.Printf("imap: listening on %v\n", addr)
	}

	return endp, nil
}

func (endp *IMAPEndpoint) Name() string {
	return "imap"
}

func (endp *IMAPEndpoint) InstanceName() string {
	return endp.name
}

func (endp *IMAPEndpoint) Version() string {
	return "0.0"
}

func (endp *IMAPEndpoint) Close() error {
	return endp.serv.Close()
}

func (endp *IMAPEndpoint) Login(username, password string) (imapbackend.User, error) {
	if !endp.Auth.CheckPlain(username, password) {
		return nil, imapbackend.ErrInvalidCredentials
	}

	return endp.Store.GetUser(username)
}

func setIMAPErrors(instName string, d caddyfile.Dispenser, s *imapserver.Server) error {
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

			s.ErrorLog = log.New(w, "imap "+instName+" ", log.LstdFlags)
		}
	}

	return nil
}

func init() {
	module.Register("imap", NewIMAPEndpoint, []string{"auth", "storage", "tls", "iodebug", "errors", "insecureauth"})
}
