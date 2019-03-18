package maddy

import (
	"crypto/tls"
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
	var err error
	endp := new(IMAPEndpoint)
	endp.name = instName
	var tlsConf *tls.Config

	insecureAuth := false
	ioDebug := false
	var errorArgs []string
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
		case "insecureauth":
			log.Printf("imap %v: insecure auth is on!\n", instName)
			insecureAuth = true
		case "iodebug":
			log.Printf("imap %v: I/O debugging is on!\n", instName)
			ioDebug = true
		case "errors":
			errorArgs = entry.Args
		}
	}

	var addresses []Address
	for _, addr := range cfg.Args {
		saddr, err := standardizeAddress(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid address: %s", instName)
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

		if addr.IsTLS() {
			l = tls.NewListener(l, tlsConf)
		}

		go func() {
			module.WaitGroup.Add(1)
			if err := endp.serv.Serve(l); err != nil {
				log.Printf("imap: failed to listen on %v: %v\n", addr, err)
			} else {
				log.Printf("imap: listening on %v\n", addr)
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
