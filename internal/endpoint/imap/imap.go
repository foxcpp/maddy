/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package imap

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/emersion/go-imap"
	compress "github.com/emersion/go-imap-compress"
	sortthread "github.com/emersion/go-imap-sortthread"
	imapbackend "github.com/emersion/go-imap/backend"
	imapserver "github.com/emersion/go-imap/server"
	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-sasl"
	i18nlevel "github.com/foxcpp/go-imap-i18nlevel"
	namespace "github.com/foxcpp/go-imap-namespace"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	tls2 "github.com/foxcpp/maddy/framework/config/tls"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/auth"
	"github.com/foxcpp/maddy/internal/authz"
	"github.com/foxcpp/maddy/internal/proxy_protocol"
	"github.com/foxcpp/maddy/internal/updatepipe"
)

type Endpoint struct {
	addrs         []string
	serv          *imapserver.Server
	proxyProtocol *proxy_protocol.ProxyProtocol
	Store         module.Storage
	tlsConfig     *tls.Config

	endpoints   []config.Endpoint
	listeners   []net.Listener
	listenersWg sync.WaitGroup

	saslAuth auth.SASLAuth

	storageNormalize authz.NormalizeFunc
	storageMap       module.Table

	Log log.Logger
}

func New(modName string, addrs []string) (module.LifetimeModule, error) {
	endp := &Endpoint{
		addrs: addrs,
		Log:   log.Logger{Name: modName},
		saslAuth: auth.SASLAuth{
			Log: log.Logger{Name: modName + "/sasl"},
		},
	}

	return endp, nil
}

func (endp *Endpoint) Configure(_ []string, cfg *config.Map) error {
	var (
		insecureAuth bool
		ioDebug      bool
		ioErrors     bool
	)

	cfg.Callback("auth", func(m *config.Map, node config.Node) error {
		return endp.saslAuth.AddProvider(m, node)
	})
	cfg.Bool("sasl_login", false, false, &endp.saslAuth.EnableLogin)
	cfg.Custom("storage", false, true, nil, modconfig.StorageDirective, &endp.Store)
	cfg.Custom("tls", true, true, nil, tls2.TLSDirective, &endp.tlsConfig)
	cfg.Custom("proxy_protocol", false, false, nil, proxy_protocol.ProxyProtocolDirective, &endp.proxyProtocol)
	cfg.Bool("insecure_auth", false, false, &insecureAuth)
	cfg.Bool("io_debug", false, false, &ioDebug)
	cfg.Bool("io_errors", false, false, &ioErrors)
	cfg.Bool("debug", true, false, &endp.Log.Debug)
	config.EnumMapped(cfg, "storage_map_normalize", false, false, authz.NormalizeFuncs, authz.NormalizeAuto,
		&endp.storageNormalize)
	modconfig.Table(cfg, "storage_map", false, false, nil, &endp.storageMap)
	config.EnumMapped(cfg, "auth_map_normalize", true, false, authz.NormalizeFuncs, authz.NormalizeAuto,
		&endp.saslAuth.AuthNormalize)
	modconfig.Table(cfg, "auth_map", true, false, nil, &endp.saslAuth.AuthMap)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	addresses := make([]config.Endpoint, 0, len(endp.addrs))
	for _, addr := range endp.addrs {
		saddr, err := config.ParseEndpoint(addr)
		if err != nil {
			return fmt.Errorf("imap: invalid address: %s", addr)
		}
		if saddr.IsTLS() && endp.tlsConfig == nil {
			return errors.New("imap: can't bind on IMAPS endpoint without TLS configuration")
		}
		addresses = append(addresses, saddr)
	}
	endp.endpoints = addresses

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
			return endp.saslAuth.CreateSASL(mech, c.Info().RemoteAddr, func(identity string, data auth.ContextData) error {
				return endp.openAccount(c, identity)
			})
		})
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

func (endp *Endpoint) Start() error {
	if updBe, ok := endp.Store.(updatepipe.Backend); ok {
		if err := updBe.EnableUpdatePipe(updatepipe.ModeReplicate); err != nil {
			endp.Log.Error("failed to initialize updates pipe", err)
		}
	}

	if err := endp.setupListeners(endp.endpoints); err != nil {
		endp.Stop()
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

		if endp.proxyProtocol != nil {
			l = proxy_protocol.NewListener(l, endp.proxyProtocol, endp.Log)
		}

		endp.listeners = append(endp.listeners, l)
		endp.listenersWg.Add(1)
		go func() {
			defer endp.listenersWg.Done()
			if err := endp.serv.Serve(l); err != nil && !strings.HasSuffix(err.Error(), "use of closed network connection") {
				endp.Log.Printf("imap: failed to serve %s: %s", addr, err)
			}
		}()
	}

	return nil
}

func (endp *Endpoint) Name() string {
	return "imap"
}

func (endp *Endpoint) InstanceName() string {
	return "imap"
}

func (endp *Endpoint) Stop() error {
	for _, l := range endp.listeners {
		l.Close()
	}
	if err := endp.serv.Close(); err != nil {
		return err
	}
	endp.listenersWg.Wait()
	return nil
}

func (endp *Endpoint) usernameForStorage(ctx context.Context, saslUsername string) (string, error) {
	saslUsername, err := endp.storageNormalize(saslUsername)
	if err != nil {
		return "", err
	}

	if endp.storageMap == nil {
		return saslUsername, nil
	}

	mapped, ok, err := endp.storageMap.Lookup(ctx, saslUsername)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", imapbackend.ErrInvalidCredentials
	}

	if saslUsername != mapped {
		endp.Log.DebugMsg("using mapped username for storage", "username", saslUsername, "mapped_username", mapped)
	}

	return mapped, nil
}

func (endp *Endpoint) openAccount(c imapserver.Conn, identity string) error {
	username, err := endp.usernameForStorage(context.TODO(), identity)
	if err != nil {
		if errors.Is(err, imapbackend.ErrInvalidCredentials) {
			return err
		}
		endp.Log.Error("failed to determine storage account name", err, "username", username)
		return fmt.Errorf("internal server error")
	}

	u, err := endp.Store.GetOrCreateIMAPAcct(username)
	if err != nil {
		return err
	}
	ctx := c.Context()
	ctx.State = imap.AuthenticatedState
	ctx.User = u
	return nil
}

func (endp *Endpoint) Login(connInfo *imap.ConnInfo, username, password string) (imapbackend.User, error) {
	// saslAuth handles AuthMap calling.
	err := endp.saslAuth.AuthPlain(username, password)
	if err != nil {
		endp.Log.Error("authentication failed", err, "username", username, "src_ip", connInfo.RemoteAddr)
		return nil, imapbackend.ErrInvalidCredentials
	}

	storageUsername, err := endp.usernameForStorage(context.TODO(), username)
	if err != nil {
		if errors.Is(err, imapbackend.ErrInvalidCredentials) {
			return nil, err
		}
		endp.Log.Error("authentication failed due to an internal error", err, "username", username, "src_ip", connInfo.RemoteAddr)
		return nil, fmt.Errorf("internal server error")
	}

	return endp.Store.GetOrCreateIMAPAcct(storageUsername)
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
		case "I18NLEVEL=1", "I18NLEVEL=2":
			endp.serv.Enable(i18nlevel.NewExtension())
		case "SORT":
			endp.serv.Enable(sortthread.NewSortExtension())
		}
		if strings.HasPrefix(ext, "THREAD") {
			endp.serv.Enable(sortthread.NewThreadExtension())
		}
	}

	endp.serv.Enable(compress.NewExtension())
	endp.serv.Enable(namespace.NewExtension())

	return nil
}

func (endp *Endpoint) SupportedThreadAlgorithms() []sortthread.ThreadAlgorithm {
	be, ok := endp.Store.(sortthread.ThreadBackend)
	if !ok {
		return nil
	}

	return be.SupportedThreadAlgorithms()
}

func init() {
	module.RegisterEndpoint("imap", New)

	imap.CharsetReader = message.CharsetReader
}
