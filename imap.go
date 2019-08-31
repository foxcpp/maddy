package maddy

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/emersion/go-imap"
	imapbackend "github.com/emersion/go-imap/backend"
	imapserver "github.com/emersion/go-imap/server"
	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/module"

	appendlimit "github.com/emersion/go-imap-appendlimit"
	compress "github.com/emersion/go-imap-compress"
	idle "github.com/emersion/go-imap-idle"
	move "github.com/emersion/go-imap-move"
	unselect "github.com/emersion/go-imap-unselect"
	"github.com/foxcpp/go-imap-sql/children"
)

type IMAPEndpoint struct {
	name      string
	aliases   []string
	serv      *imapserver.Server
	listeners []net.Listener
	Auth      module.AuthProvider
	Store     module.Storage

	updater     imapbackend.BackendUpdater
	tlsConfig   *tls.Config
	listenersWg sync.WaitGroup

	Log log.Logger
}

func NewIMAPEndpoint(_, instName string, aliases []string) (module.Module, error) {
	endp := &IMAPEndpoint{
		name:    instName,
		aliases: aliases,
		Log:     log.Logger{Name: "imap"},
	}
	endp.name = instName

	return endp, nil
}

func (endp *IMAPEndpoint) Init(cfg *config.Map) error {
	var (
		insecureAuth bool
		ioDebug      bool
	)

	cfg.Custom("auth", false, true, nil, authDirective, &endp.Auth)
	cfg.Custom("storage", false, true, nil, storageDirective, &endp.Store)
	cfg.Custom("tls", true, true, nil, tlsDirective, &endp.tlsConfig)
	cfg.Bool("insecure_auth", false, &insecureAuth)
	cfg.Bool("io_debug", false, &ioDebug)
	cfg.Bool("debug", true, &endp.Log.Debug)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	var ok bool
	endp.updater, ok = endp.Store.(imapbackend.BackendUpdater)
	if !ok {
		return fmt.Errorf("imap: storage module %T does not implement imapbackend.BackendUpdater", endp.Store)
	}

	// Call Updates once at start, some storage backends initialize update
	// channel lazily and may not generate updates at all unless it is called.
	if endp.updater.Updates() != nil {
		return fmt.Errorf("imap: failed to init backend: nil update channel")
	}

	args := append([]string{endp.name}, endp.aliases...)
	addresses := make([]Address, 0, len(args))
	for _, addr := range args {
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
	endp.serv.ErrorLog = &endp.Log
	if ioDebug {
		endp.serv.Debug = endp.Log.DebugWriter()
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

func (endp *IMAPEndpoint) Login(connInfo *imap.ConnInfo, username, password string) (imapbackend.User, error) {
	if !endp.Auth.CheckPlain(username, password) {
		endp.Log.Printf("authentication failed for %s (from %v)", username, connInfo.RemoteAddr)
		return nil, imapbackend.ErrInvalidCredentials
	}

	return endp.Store.GetOrCreateUser(username)
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

	imap.CharsetReader = message.CharsetReader
}
