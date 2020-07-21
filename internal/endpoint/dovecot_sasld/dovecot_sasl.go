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
	"fmt"
	stdlog "log"
	"net"
	"strings"
	"sync"

	"github.com/emersion/go-sasl"
	dovecotsasl "github.com/foxcpp/go-dovecot-sasl"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/auth"
)

const modName = "dovecot_sasld"

type Endpoint struct {
	addrs    []string
	log      log.Logger
	saslAuth auth.SASLAuth

	listenersWg sync.WaitGroup

	srv *dovecotsasl.Server
}

func New(_ string, addrs []string) (module.Module, error) {
	return &Endpoint{
		addrs: addrs,
		saslAuth: auth.SASLAuth{
			Log: log.Logger{Name: modName + "/saslauth"},
		},
		log: log.Logger{Name: modName, Debug: log.DefaultLogger.Debug},
	}, nil
}

func (endp *Endpoint) Name() string {
	return modName
}

func (endp *Endpoint) InstanceName() string {
	return modName
}

func (endp *Endpoint) Init(cfg *config.Map) error {
	cfg.Callback("auth", func(m *config.Map, node config.Node) error {
		return endp.saslAuth.AddProvider(m, node)
	})
	if _, err := cfg.Process(); err != nil {
		return err
	}

	endp.srv = dovecotsasl.NewServer()
	endp.srv.Log = stdlog.New(endp.log, "", 0)

	for _, mech := range endp.saslAuth.SASLMechanisms() {
		mech := mech
		endp.srv.AddMechanism(mech, mechInfo[mech], func(req *dovecotsasl.AuthReq) sasl.Server {
			var remoteAddr net.Addr
			if req.RemoteIP != nil && req.RemotePort != 0 {
				remoteAddr = &net.TCPAddr{IP: req.RemoteIP, Port: int(req.RemotePort)}
			}

			return endp.saslAuth.CreateSASL(mech, remoteAddr, func(_ string) error { return nil })
		})
	}

	for _, addr := range endp.addrs {
		parsed, err := config.ParseEndpoint(addr)
		if err != nil {
			return fmt.Errorf("%s: %v", modName, err)
		}

		l, err := net.Listen(parsed.Network(), parsed.Address())
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

func (endp *Endpoint) Close() error {
	return endp.srv.Close()
}

func init() {
	module.RegisterEndpoint(modName, New)
}
