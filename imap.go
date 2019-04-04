package maddy

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	imapbackend "github.com/emersion/go-imap/backend"
	imapserver "github.com/emersion/go-imap/server"
	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/log"
	"github.com/emersion/maddy/module"

	"github.com/emersion/go-imap-appendlimit"
	"github.com/emersion/go-imap-compress"
	"github.com/emersion/go-imap-idle"
	"github.com/emersion/go-imap-move"
	"github.com/emersion/go-imap-unselect"
	"github.com/foxcpp/go-imap-sql/imap/children"
)

type IMAPEndpoint struct {
	name      string
	serv      *imapserver.Server
	listeners []net.Listener
	Auth      module.AuthProvider
	Store     module.Storage

	updater     imapbackend.BackendUpdater
	tlsConfig   *tls.Config
	listenersWg sync.WaitGroup

	Log log.Logger
}

func NewIMAPEndpoint(_, instName string) (module.Module, error) {
	endp := &IMAPEndpoint{
		name: instName,
		Log:  log.Logger{Out: log.StderrLog, Name: "imap"},
	}
	endp.name = instName

	return endp, nil
}

func (endp *IMAPEndpoint) Init(globalCfg map[string]config.Node, rawCfg config.Node) error {
	var (
		insecureAuth bool
		ioDebug      bool
	)

	cfg := config.Map{}
	cfg.Custom("auth", false, false, defaultAuthProvider, authDirective, &endp.Auth)
	cfg.Custom("storage", false, false, defaultStorage, storageDirective, &endp.Store)
	cfg.Custom("tls", true, true, nil, tlsDirective, &endp.tlsConfig)
	cfg.Bool("insecure_auth", false, &insecureAuth)
	cfg.Bool("io_debug", false, &ioDebug)
	cfg.Bool("debug", true, &endp.Log.Debug)
	if _, err := cfg.Process(globalCfg, &rawCfg); err != nil {
		return err
	}

	var ok bool
	endp.updater, ok = endp.Store.(imapbackend.BackendUpdater)
	if !ok {
		return fmt.Errorf("imap: storage module %T does not implement imapbackend.BackendUpdater", endp.Store)
	}

	addresses := make([]Address, 0, len(rawCfg.Args))
	for _, addr := range rawCfg.Args {
		saddr, err := standardizeAddress(addr)
		if err != nil {
			return fmt.Errorf("imap: invalid address: %s", endp.name)
		}
		if saddr.Scheme != "imap" && saddr.Scheme != "imaps" {
			return fmt.Errorf("imap: imap or imaps scheme must be used, got %s", saddr.Scheme)
		}
		addresses = append(addresses, saddr)
	}

	endp.serv = imapserver.New(endp)
	endp.serv.AllowInsecureAuth = insecureAuth
	endp.serv.TLSConfig = endp.tlsConfig
	endp.serv.ErrorLog = endp.Log.DebugWriter()
	if ioDebug {
		endp.serv.Debug = os.Stderr
		endp.Log.Println("I/O debugging is on! It may leak passwords in logs, be careful!")
	}

	if err := endp.enableExtensions(); err != nil {
		return err
	}

	for _, addr := range addresses {
		var l net.Listener
		var err error
		l, err = net.Listen(addr.Network(), addr.Address())
		if err != nil {
			return fmt.Errorf("failed to bind on %v: %v", addr, err)
		}
		endp.Log.Printf("listening on %v", addr)

		if addr.IsTLS() {
			if endp.tlsConfig == nil {
				return errors.New("can't bind on IMAPS endpoint without TLS configuration")
			}
			l = tls.NewListener(l, endp.tlsConfig)
		}

		endp.listeners = append(endp.listeners, l)

		endp.listenersWg.Add(1)
		addr := addr
		go func() {
			if err := endp.serv.Serve(l); err != nil && !strings.HasSuffix(err.Error(), "use of closed network connection") {
				endp.Log.Printf("imap: failed to serve %s: %s", addr, err)
			}
			endp.listenersWg.Done()
		}()
	}

	if endp.serv.AllowInsecureAuth {
		endp.Log.Println("authentication over unencrypted connections is allowed, this is insecure configuration and should be used only for testing!")
	}
	if endp.serv.TLSConfig == nil {
		endp.Log.Println("TLS is disabled, this is insecure configuration and should be used only for testing!")
		endp.serv.AllowInsecureAuth = true
	}

	return nil
}

func (endp *IMAPEndpoint) Updates() <-chan imapbackend.Update {
	return endp.updater.Updates()
}

func (endp *IMAPEndpoint) Name() string {
	return "imap"
}

func (endp *IMAPEndpoint) InstanceName() string {
	return endp.name
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
		endp.Log.Printf("authentication failed for %s", username)
		return nil, imapbackend.ErrInvalidCredentials
	}

	return endp.Store.GetUser(username)
}

func (endp *IMAPEndpoint) EnableChildrenExt() bool {
	return endp.Store.(children.Backend).EnableChildrenExt()
}

func (endp *IMAPEndpoint) enableExtensions() error {
	exts := endp.Store.IMAPExtensions()
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

	endp.serv.Enable(compress.NewExtension())
	endp.serv.Enable(unselect.NewExtension())
	endp.serv.Enable(idle.NewExtension())

	return nil
}

func init() {
	module.Register("imap", NewIMAPEndpoint)
}
