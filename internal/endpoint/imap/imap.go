package imap

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/emersion/go-imap"
	appendlimit "github.com/emersion/go-imap-appendlimit"
	compress "github.com/emersion/go-imap-compress"
	idle "github.com/emersion/go-imap-idle"
	move "github.com/emersion/go-imap-move"
	specialuse "github.com/emersion/go-imap-specialuse"
	unselect "github.com/emersion/go-imap-unselect"
	imapbackend "github.com/emersion/go-imap/backend"
	imapserver "github.com/emersion/go-imap/server"
	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-sasl"
	i18nlevel "github.com/foxcpp/go-imap-i18nlevel"
	"github.com/foxcpp/go-imap-sql/children"
	"github.com/foxcpp/maddy/internal/auth"
	"github.com/foxcpp/maddy/internal/config"
	modconfig "github.com/foxcpp/maddy/internal/config/module"
	"github.com/foxcpp/maddy/internal/log"
	"github.com/foxcpp/maddy/internal/module"
	"github.com/foxcpp/maddy/internal/updatepipe"
)

type Endpoint struct {
	addrs     []string
	serv      *imapserver.Server
	listeners []net.Listener
	Store     module.Storage

	updater     imapbackend.BackendUpdater
	tlsConfig   *tls.Config
	listenersWg sync.WaitGroup

	saslAuth auth.SASLAuth

	Log log.Logger
}

func New(modName string, addrs []string) (module.Module, error) {
	endp := &Endpoint{
		addrs: addrs,
		Log:   log.Logger{Name: "imap"},
		saslAuth: auth.SASLAuth{
			Log: log.Logger{Name: "imap/saslauth"},
		},
	}

	return endp, nil
}

func (endp *Endpoint) Init(cfg *config.Map) error {
	var (
		insecureAuth bool
		ioDebug      bool
		ioErrors     bool
	)

	cfg.Callback("auth", func(m *config.Map, node *config.Node) error {
		return endp.saslAuth.AddProvider(m, node)
	})
	cfg.Custom("storage", false, true, nil, modconfig.StorageDirective, &endp.Store)
	cfg.Custom("tls", true, true, nil, config.TLSDirective, &endp.tlsConfig)
	cfg.Bool("insecure_auth", false, false, &insecureAuth)
	cfg.Bool("io_debug", false, false, &ioDebug)
	cfg.Bool("io_errors", false, false, &ioErrors)
	cfg.Bool("debug", true, false, &endp.Log.Debug)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	var ok bool
	endp.updater, ok = endp.Store.(imapbackend.BackendUpdater)
	if !ok {
		return fmt.Errorf("imap: storage module %T does not implement imapbackend.BackendUpdater", endp.Store)
	}

	if updBe, ok := endp.Store.(updatepipe.Backend); ok {
		if err := updBe.EnableUpdatePipe(updatepipe.ModeReplicate); err != nil {
			endp.Log.Error("failed to initialize updates pipe", err)
		}
	}

	// Call Updates once at start, some storage backends initialize update
	// channel lazily and may not generate updates at all unless it is called.
	if endp.updater.Updates() == nil {
		return fmt.Errorf("imap: failed to init backend: nil update channel")
	}

	addresses := make([]config.Endpoint, 0, len(endp.addrs))
	for _, addr := range endp.addrs {
		saddr, err := config.ParseEndpoint(addr)
		if err != nil {
			return fmt.Errorf("imap: invalid address: %s", addr)
		}
		addresses = append(addresses, saddr)
	}

	endp.serv = imapserver.New(endp)
	endp.serv.AllowInsecureAuth = insecureAuth
	endp.serv.TLSConfig = endp.tlsConfig
	if ioErrors {
		endp.serv.ErrorLog = &endp.Log
	} else {
		endp.serv.ErrorLog = log.Logger{Out: log.NopOutput{}}
	}
	if ioDebug {
		endp.serv.Debug = endp.Log.DebugWriter()
		endp.Log.Println("I/O debugging is on! It may leak passwords in logs, be careful!")
	}

	if err := endp.enableExtensions(); err != nil {
		return err
	}

	for _, mech := range endp.saslAuth.SASLMechanisms() {
		endp.serv.EnableAuth(mech, func(c imapserver.Conn) sasl.Server {
			return endp.saslAuth.CreateSASL(mech, c.Info().RemoteAddr, func(ids []string) error {
				return endp.openAccount(c, ids)
			})
		})
	}

	if err := endp.setupListeners(addresses); err != nil {
		return err
	}

	return nil
}

func (endp *Endpoint) setupListeners(addresses []config.Endpoint) error {
	for _, addr := range addresses {
		var l net.Listener
		var err error
		l, err = net.Listen(addr.Network(), addr.Address())
		if err != nil {
			return fmt.Errorf("imap: %v", err)
		}
		endp.Log.Printf("listening on %v", addr)

		if addr.IsTLS() {
			if endp.tlsConfig == nil {
				return errors.New("imap: can't bind on IMAPS endpoint without TLS configuration")
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

func (endp *Endpoint) Updates() <-chan imapbackend.Update {
	return endp.updater.Updates()
}

func (endp *Endpoint) Name() string {
	return "imap"
}

func (endp *Endpoint) InstanceName() string {
	return "imap"
}

func (endp *Endpoint) Close() error {
	for _, l := range endp.listeners {
		l.Close()
	}
	if err := endp.serv.Close(); err != nil {
		return err
	}
	endp.listenersWg.Wait()
	return nil
}

func (endp *Endpoint) openAccount(c imapserver.Conn, identities []string) error {
	u, err := endp.Store.GetOrCreateUser(identities[0])
	if err != nil {
		return err
	}
	ctx := c.Context()
	ctx.State = imap.AuthenticatedState
	ctx.User = u
	return nil
}

func (endp *Endpoint) Login(connInfo *imap.ConnInfo, username, password string) (imapbackend.User, error) {
	_, err := endp.saslAuth.AuthPlain(username, password)
	if err != nil {
		endp.Log.Error("authentication failed", err, "username", username, "src_ip", connInfo.RemoteAddr)
		return nil, imapbackend.ErrInvalidCredentials
	}

	// TODO: Wrap GetOrCreateUser and possibly implement INBOXES extension
	// (though it is draft 00 for quite some time so it likely has no future).
	return endp.Store.GetOrCreateUser(username)
}

func (endp *Endpoint) EnableChildrenExt() bool {
	return endp.Store.(children.Backend).EnableChildrenExt()
}

func (endp *Endpoint) I18NLevel() int {
	be, ok := endp.Store.(i18nlevel.Backend)
	if !ok {
		return 0
	}
	return be.I18NLevel()
}

func (endp *Endpoint) enableExtensions() error {
	exts := endp.Store.IMAPExtensions()
	for _, ext := range exts {
		switch ext {
		case "APPENDLIMIT":
			endp.serv.Enable(appendlimit.NewExtension())
		case "CHILDREN":
			endp.serv.Enable(children.NewExtension())
		case "MOVE":
			endp.serv.Enable(move.NewExtension())
		case "SPECIAL-USE":
			endp.serv.Enable(specialuse.NewExtension())
		case "I18NLEVEL=1", "I18NLEVEL=2":
			endp.serv.Enable(i18nlevel.NewExtension())
		}
	}

	endp.serv.Enable(compress.NewExtension())
	endp.serv.Enable(unselect.NewExtension())
	endp.serv.Enable(idle.NewExtension())

	return nil
}

func init() {
	module.RegisterEndpoint("imap", New)

	imap.CharsetReader = message.CharsetReader
}
