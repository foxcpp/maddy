//go:build !no_imap1

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
	"errors"
	"fmt"
	"strings"

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
	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/framework/module/modules"
	"github.com/foxcpp/maddy/internal/auth"
	"github.com/foxcpp/maddy/internal/authz"
	"github.com/foxcpp/maddy/internal/endpoint"
	"github.com/foxcpp/maddy/internal/updatepipe"
)

type Endpoint struct {
	ln *endpoint.MetaListener

	serv     *imapserver.Server
	Store    module.Storage
	saslAuth auth.SASLAuth

	storageNormalize authz.NormalizeFunc
	storageMap       module.Table

	log *log.Logger
}

func New(c *container.C, modName string, addrs []string) (container.LifetimeModule, error) {
	logger := c.DefaultLogger.Sublogger(modName)
	endp := &Endpoint{
		log: logger,
		saslAuth: auth.SASLAuth{
			Log: logger.Sublogger("sasl"),
		},
	}

	endp.ln = endpoint.NewMetaListener(endp.log.Sublogger("listen"), addrs)

	return endp, nil
}

func (endp *Endpoint) Configure(_ []string, cfg *config.Map) error {
	var (
		insecureAuth bool
		ioDebug      bool
		ioErrors     bool
	)

	endp.ln.AddConfig(cfg)
	cfg.Callback("auth", func(m *config.Map, node config.Node) error {
		return endp.saslAuth.AddProvider(m, node)
	})
	cfg.Bool("sasl_login", false, false, &endp.saslAuth.EnableLogin)
	cfg.Custom("storage", false, true, nil, modconfig.StorageDirective, &endp.Store)
	cfg.Bool("insecure_auth", false, false, &insecureAuth)
	cfg.Bool("io_debug", false, false, &ioDebug)
	cfg.Bool("io_errors", false, false, &ioErrors)
	cfg.Bool("debug", true, false, &endp.log.Debug)
	config.EnumMapped(cfg, "storage_map_normalize", false, false, authz.NormalizeFuncs, authz.NormalizeAuto,
		&endp.storageNormalize)
	modconfig.Table(cfg, "storage_map", false, false, nil, &endp.storageMap)
	config.EnumMapped(cfg, "auth_map_normalize", true, false, authz.NormalizeFuncs, authz.NormalizeAuto,
		&endp.saslAuth.AuthNormalize)
	modconfig.Table(cfg, "auth_map", true, false, nil, &endp.saslAuth.AuthMap)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	endp.saslAuth.Log.Debug = endp.log.Debug

	if err := endp.ln.ParseEndpoints(); err != nil {
		return err
	}

	endp.serv = imapserver.New(endp)
	endp.serv.AllowInsecureAuth = insecureAuth
	endp.serv.TLSConfig = endp.ln.TLSConfig
	if ioErrors {
		endp.serv.ErrorLog = endp.log
	} else {
		endp.serv.ErrorLog = &log.NopLogger
	}
	if ioDebug {
		endp.serv.Debug = endp.log.DebugWriter()
		endp.log.Println("I/O debugging is on! It may leak passwords in logs, be careful!")
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
		endp.log.Println("authentication over unencrypted connections is allowed, this is insecure configuration and should be used only for testing!")
	}
	if endp.serv.TLSConfig == nil {
		endp.log.Println("TLS is disabled, this is insecure configuration and should be used only for testing!")
		endp.serv.AllowInsecureAuth = true
	}

	return nil
}

func (endp *Endpoint) Start() error {
	if updBe, ok := endp.Store.(updatepipe.Backend); ok {
		if err := updBe.EnableUpdatePipe(updatepipe.ModeReplicate); err != nil {
			endp.log.Error("failed to initialize updates pipe", err)
		}
	}

	if err := endp.ln.ListenAndServe(endp.serv); err != nil {
		if err := endp.Stop(); err != nil {
			endp.log.Error("failed to stop after setupListeners error", err)
		}
		return err
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
	return endp.ln.Close()
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
		endp.log.DebugMsg("using mapped username for storage", "username", saslUsername, "mapped_username", mapped)
	}

	return mapped, nil
}

func (endp *Endpoint) openAccount(c imapserver.Conn, identity string) error {
	username, err := endp.usernameForStorage(context.TODO(), identity)
	if err != nil {
		if errors.Is(err, imapbackend.ErrInvalidCredentials) {
			return err
		}
		endp.log.Error("failed to determine storage account name", err, "username", username)
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
		endp.log.Error("authentication failed", err, "username", username, "src_ip", connInfo.RemoteAddr)
		return nil, imapbackend.ErrInvalidCredentials
	}

	storageUsername, err := endp.usernameForStorage(context.TODO(), username)
	if err != nil {
		if errors.Is(err, imapbackend.ErrInvalidCredentials) {
			return nil, err
		}
		endp.log.Error("authentication failed due to an internal error", err, "username", username, "src_ip", connInfo.RemoteAddr)
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
	modules.RegisterEndpoint("imap", New)

	imap.CharsetReader = message.CharsetReader
}
