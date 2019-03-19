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
	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/module"
)

type IMAPEndpoint struct {
	name  string
	serv  *imapserver.Server
	Auth  module.AuthProvider
	Store module.Storage
}

func NewIMAPEndpoint(instName string, cfg config.CfgTreeNode) (module.Module, error) {
	endp := new(IMAPEndpoint)
	endp.name = instName
	var (
		err          error
		tlsConf      *tls.Config
		tlsSet       bool
		errorArgs    []string
		insecureAuth bool
		ioDebug      bool
	)

	for _, entry := range cfg.Childrens {
		switch entry.Name {
		case "auth":
			endp.Auth, err = authProvider(entry.Args)
			if err != nil {
				return nil, err
			}
		case "storage":
			endp.Store, err = storageBackend(entry.Args)
			if err != nil {
				return nil, err
			}
		case "tls":
			if err := setTLS(entry.Args, &tlsConf); err != nil {
				return nil, err
			}
			tlsSet = true
		case "insecureauth":
			log.Printf("imap %v: insecure auth is on!\n", instName)
			insecureAuth = true
		case "iodebug":
			log.Printf("imap %v: I/O debugging is on!\n", instName)
			ioDebug = true
		case "errors":
			errorArgs = entry.Args
		default:
			return nil, fmt.Errorf("unknown config directive: %s", entry.Name)
		}
	}

	if !tlsSet {
		return nil, fmt.Errorf("TLS is not configured")
	}

	if endp.Auth == nil {
		endp.Auth, err = authProvider([]string{"default-auth"})
		if err != nil {
			endp.Auth, err = authProvider([]string{"default"})
			if err != nil {
				return nil, errors.New("missing default auth. provider, must set custom")
			}
		}
		log.Printf("imap %s: using %s auth. provider (%s %s)",
			instName, endp.Auth.(module.Module).InstanceName(),
			endp.Auth.(module.Module).Name(), endp.Auth.(module.Module).Version(),
		)
	}
	if endp.Store == nil {
		endp.Store, err = storageBackend([]string{"default-storage"})
		if err != nil {
			endp.Store, err = storageBackend([]string{"default"})
			if err != nil {
				return nil, errors.New("missing default storage backend, must set custom")
			}
		}
		log.Printf("imap %s: using %s storage backend (%s %s)",
			instName, endp.Store.(module.Module).InstanceName(),
			endp.Store.(module.Module).Name(), endp.Store.(module.Module).Version(),
		)
	}

	var addresses []Address
	for _, addr := range cfg.Args {
		saddr, err := standardizeAddress(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid address: %s", instName)
		}
		if saddr.Scheme != "imap" && saddr.Scheme != "imaps" {
			return nil, fmt.Errorf("imap %s: imap or imaps scheme must be used, got %s", instName, saddr.Scheme)
		}
		addresses = append(addresses, saddr)
	}

	endp.serv = imapserver.New(endp)
	endp.serv.AllowInsecureAuth = insecureAuth
	if ioDebug {
		endp.serv.Debug = os.Stderr
	}
	if errorArgs != nil {
		err := setIMAPErrors(instName, errorArgs, endp.serv)
		if err != nil {
			return nil, err
		}
	}

	for _, addr := range addresses {
		var l net.Listener
		var err error
		l, err = net.Listen(addr.Network(), addr.Address())
		if err != nil {
			return nil, fmt.Errorf("failed to bind on %v: %v", addr, err)
		}
		log.Printf("imap: listening on %v\n", addr)

		if addr.IsTLS() {
			l = tls.NewListener(l, tlsConf)
		}

		go func() {
			module.WaitGroup.Add(1)
			if err := endp.serv.Serve(l); err != nil {
				log.Printf("imap: failed to listen on %v: %v\n", addr, err)
			}
			module.WaitGroup.Done()
		}()
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
	return VersionStr
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

func setIMAPErrors(instName string, args []string, s *imapserver.Server) error {
	if len(args) != 1 {
		return fmt.Errorf("missing errors directive values")
	}

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

	return nil
}

func init() {
	module.Register("imap", NewIMAPEndpoint)
}
