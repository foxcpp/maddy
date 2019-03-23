package maddy

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"sync"

	appendlimit "github.com/emersion/go-imap-appendlimit"
	move "github.com/emersion/go-imap-move"
	imapbackend "github.com/emersion/go-imap/backend"
	imapserver "github.com/emersion/go-imap/server"
	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/module"
	"github.com/foxcpp/go-sqlmail/imap/children"
)

type IMAPEndpoint struct {
	name      string
	serv      *imapserver.Server
	listeners []net.Listener
	Auth      module.AuthProvider
	Store     module.Storage

	tlsConfig   *tls.Config
	listenersWg sync.WaitGroup
}

func NewIMAPEndpoint(instName string, globalCfg map[string][]string, cfg config.Node) (module.Module, error) {
	endp := new(IMAPEndpoint)
	endp.name = instName
	var (
		err          error
		errorArgs    []string
		tlsArgs      []string
		insecureAuth bool
		ioDebug      bool
	)

	for _, entry := range cfg.Children {
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
			tlsArgs = entry.Args
		case "insecure_auth":
			log.Printf("imap %s: authentication over unencrypted connections is allowed, this is insecure configuration and should be used only for testing!", instName)
			insecureAuth = true
		case "io_debug":
			log.Printf("imap %s: I/O debugging is on!", instName)
			ioDebug = true
		case "errors":
			errorArgs = entry.Args
		default:
			return nil, fmt.Errorf("unknown config directive: %s", entry.Name)
		}
	}

	if globalTls, ok := globalCfg["tls"]; tlsArgs == nil && ok {
		tlsArgs = globalTls
	}
	if tlsArgs == nil {
		return nil, errors.New("TLS is not configured")
	}
	endp.tlsConfig = new(tls.Config)
	if err := setTLS(tlsArgs, &endp.tlsConfig); err != nil {
		return nil, err
	}
	// Print warning only if TLS is in per-module configuration.
	// Otherwise we will print it when reading global config.
	if endp.tlsConfig == nil && globalCfg["tls"] == nil {
		log.Printf("imap %s: TLS is disabled, this is insecure configuration and should be used only for testing!", endp.name)
		insecureAuth = true
	}

	if endp.Auth == nil {
		endp.Auth, err = authProvider([]string{"default_auth"})
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
		endp.Store, err = storageBackend([]string{"default_storage"})
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

	addresses := make([]Address, 0, len(cfg.Args))
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

	if err := endp.enableExtensions(); err != nil {
		return nil, err
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
			l = tls.NewListener(l, endp.tlsConfig)
		}

		endp.listeners = append(endp.listeners, l)

		endp.listenersWg.Add(1)
		addr := addr
		go func() {
			if err := endp.serv.Serve(l); err != nil && !strings.HasSuffix(err.Error(), "use of closed network connection") {
				log.Printf("imap: failed to serve %s: %s\n", addr, err)
			}
			endp.listenersWg.Done()
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
	for _, l := range endp.listeners {
		l.Close()
	}
	if err := endp.serv.Close(); err != nil {
		return err
	}
	endp.listenersWg.Wait()
	return nil
}

func (endp *IMAPEndpoint) Login(username, password string) (imapbackend.User, error) {
	if !endp.Auth.CheckPlain(username, password) {
		return nil, imapbackend.ErrInvalidCredentials
	}

	return endp.Store.GetUser(username)
}

func (endp *IMAPEndpoint) EnableChildrenExt() bool {
	return endp.Store.(children.Backend).EnableChildrenExt()
}

func (endp *IMAPEndpoint) enableExtensions() error {
	exts := endp.Store.Extensions()
	for _, ext := range exts {
		switch ext {
		case "APPENDLIMIT":
			endp.serv.Enable(appendlimit.NewExtension())
		case "CHILDREN":
			endp.serv.Enable(children.NewExtension())
		case "MOVE":
			endp.serv.Enable(move.NewExtension())
		}
	}
	return nil
}

func setIMAPErrors(instName string, args []string, s *imapserver.Server) error {
	if len(args) != 1 {
		return fmt.Errorf("missing errors directive values")
	}

	output := args[0]
	var w io.Writer
	switch output {
	case "off":
		w = ioutil.Discard
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
