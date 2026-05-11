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

package dovecotsasld

import (
	"crypto/tls"
	"fmt"
	stdlog "log"
	"net"
	"strings"
	"sync"

	"github.com/emersion/go-sasl"
	dovecotsasl "github.com/foxcpp/go-dovecot-sasl"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/framework/module/modules"
	"github.com/foxcpp/maddy/framework/resource/netresource"
	"github.com/foxcpp/maddy/internal/auth"
	"github.com/foxcpp/maddy/internal/authz"
)

const modName = "dovecot_sasld"

type Endpoint struct {
	addrs    []string
	log      *log.Logger
	saslAuth auth.SASLAuth

	endpoints   []config.Endpoint
	listenersWg sync.WaitGroup

	srv *dovecotsasl.Server
}

func New(c *container.C, _ string, addrs []string) (container.LifetimeModule, error) {
	logger := c.DefaultLogger.Sublogger(modName)
	return &Endpoint{
		addrs: addrs,
		saslAuth: auth.SASLAuth{
			Log: logger.Sublogger("sasl"),
		},
		log: logger,
	}, nil
}

func (endp *Endpoint) Name() string {
	return modName
}

func (endp *Endpoint) InstanceName() string {
	return modName
}

func proxiedTLSData(req *dovecotsasl.AuthReq) *module.ProxiedTLSContext {
	var version uint16
	switch req.TLSProtocol {
	case "TLSv1.0":
		version = tls.VersionTLS10
	case "TLSv1.1":
		version = tls.VersionTLS11
	case "TLSv1.2":
		version = tls.VersionTLS12
	case "TLSv1.3":
		version = tls.VersionTLS13
	default:
		version = tls.VersionTLS10
	}

	return &module.ProxiedTLSContext{
		ValidCert:    req.ValidClientCert,
		CertUsername: req.CertUsername,
		Cipher:       req.TLSCipher,
		CipherBits:   req.TLSCipherBits,
		PFS:          req.TLSPFS,
		Version:      version,
	}
}

func (endp *Endpoint) Configure(_ []string, cfg *config.Map) error {
	cfg.Callback("auth", func(m *config.Map, node config.Node) error {
		return endp.saslAuth.AddProvider(m, node)
	})
	cfg.Bool("sasl_login", false, false, &endp.saslAuth.EnableLogin)
	config.EnumMapped(cfg, "auth_map_normalize", true, false, authz.NormalizeFuncs, authz.NormalizeAuto,
		&endp.saslAuth.AuthNormalize)
	modconfig.Table(cfg, "auth_map", true, false, nil, &endp.saslAuth.AuthMap)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	endp.srv = dovecotsasl.NewServer()
	endp.saslAuth.Log.Debug = endp.log.Debug
	endp.srv.Log = stdlog.New(endp.log, "", 0)

	for _, mech := range endp.saslAuth.SASLMechanisms() {
		endp.srv.AddMechanism(mech, mechInfo[mech], func(req *dovecotsasl.AuthReq) sasl.Server {
			var remoteAddr net.Addr
			if req.RemoteIP != nil && req.RemotePort != 0 {
				remoteAddr = &net.TCPAddr{IP: req.RemoteIP, Port: int(req.RemotePort)}
			}

			var localAddr net.Addr
			if req.LocalIP != nil && req.LocalPort != 0 {
				localAddr = &net.TCPAddr{IP: req.LocalIP, Port: int(req.LocalPort)}
			}

			var proxiedTLS *module.ProxiedTLSContext
			if (req.Secured && req.SecuredMethod == dovecotsasl.SecuredTLS) ||
				req.Transport == string(dovecotsasl.TransportTLS) {
				proxiedTLS = proxiedTLSData(req)
			}

			return endp.saslAuth.CreateSASL(mech, &auth.SASLContext{
				Service:    req.Service,
				LocalAddr:  localAddr,
				RemoteAddr: remoteAddr,
				ProxiedTLS: proxiedTLS,
			}, func(_ string, _ auth.ContextData) error { return nil })
		})
	}

	for _, addr := range endp.addrs {
		parsed, err := config.ParseEndpoint(addr)
		if err != nil {
			return fmt.Errorf("%s: %v", modName, err)
		}

		endp.endpoints = append(endp.endpoints, parsed)
	}

	return nil
}

func (endp *Endpoint) Start() error {
	for _, addr := range endp.endpoints {
		l, err := netresource.Listen(addr.Network(), addr.Address())
		if err != nil {
			return fmt.Errorf("%s: %v", modName, err)
		}

		endp.log.Printf("listening on %v", l.Addr())
		endp.listenersWg.Add(1)
		go func() {
			defer endp.listenersWg.Done()
			if err := endp.srv.Serve(l); err != nil {
				if !strings.HasSuffix(err.Error(), "use of closed network connection") {
					endp.log.Printf("failed to serve %v: %v", l.Addr(), err)
				}
			}
		}()
	}
	return nil
}

func (endp *Endpoint) Stop() error {
	defer endp.listenersWg.Wait()
	return endp.srv.Close()
}

func init() {
	modules.RegisterEndpoint(modName, New)
}
