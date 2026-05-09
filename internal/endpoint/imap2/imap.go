/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2026 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

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

package imap2

import (
	"context"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	_ "github.com/emersion/go-message/charset"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/framework/module/modules"
	"github.com/foxcpp/maddy/framework/module/storage"
	"github.com/foxcpp/maddy/internal/auth"
	"github.com/foxcpp/maddy/internal/authz"
	"github.com/foxcpp/maddy/internal/endpoint"
	"github.com/foxcpp/maddy/internal/updatepipe"
)

type Endpoint struct {
	ln *endpoint.MetaListener

	serv  *imapserver.Server
	Store storage.Storage

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
	modconfig.Module[storage.Storage](cfg, "storage", false, true, "storage", nil, &endp.Store)
	cfg.Bool("insecure_auth", false, false, &insecureAuth)
	cfg.Bool("io_debug", false, false, &ioDebug)
	cfg.Bool("io_errors", false, true, &ioErrors)
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

	if insecureAuth {
		endp.log.Println("authentication over unencrypted connections is allowed, this is insecure configuration and should be used only for testing!")
	}
	if endp.ln.TLSConfig == nil {
		endp.log.Println("TLS is disabled, this is insecure configuration and should be used only for testing!")
		insecureAuth = true
	}

	endp.saslAuth.Log.Debug = endp.log.Debug

	if err := endp.ln.ParseEndpoints(); err != nil {
		return err
	}

	opts := &imapserver.Options{
		NewSession:   endp.NewSession,
		Caps:         endp.AvailableCaps(),
		Logger:       nil,
		TLSConfig:    endp.ln.TLSConfig,
		InsecureAuth: insecureAuth,
		DebugWriter:  nil,
	}
	if ioErrors {
		opts.Logger = endp.log
	} else {
		opts.Logger = &log.Logger{Out: log.NopOutput{}}
	}
	if ioDebug {
		opts.DebugWriter = endp.log.DebugWriter()
		endp.log.Println("I/O debugging is on! It may leak passwords in logs, be careful!")
	}
	endp.serv = imapserver.New(opts)

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
	return "imap2"
}

func (endp *Endpoint) InstanceName() string {
	return "imap2"
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
		return "", imapserver.ErrAuthFailed
	}

	if saslUsername != mapped {
		endp.log.DebugMsg("using mapped username for storage", "username", saslUsername, "mapped_username", mapped)
	}

	return mapped, nil
}

func (endp *Endpoint) AvailableCaps() imap.CapSet {
	return endp.Store.AvailableCaps()
}

func init() {
	modules.RegisterEndpoint("imap2", New)
}
