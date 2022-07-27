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

package openmetrics

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const modName = "openmetrics"

type Endpoint struct {
	addrs  []string
	logger log.Logger

	listenersWg sync.WaitGroup
	serv        http.Server
	mux         *http.ServeMux
}

func New(_ string, args []string) (module.Module, error) {
	return &Endpoint{
		addrs:  args,
		logger: log.Logger{Name: modName, Debug: log.DefaultLogger.Debug},
	}, nil
}

func (e *Endpoint) Init(cfg *config.Map) error {
	cfg.Bool("debug", false, false, &e.logger.Debug)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	e.mux = http.NewServeMux()
	e.mux.Handle("/metrics", promhttp.Handler())
	e.serv.Handler = e.mux

	for _, a := range e.addrs {
		a := a
		endp, err := config.ParseEndpoint(a)
		if err != nil {
			return fmt.Errorf("%s: malformed endpoint: %v", modName, err)
		}
		if endp.IsTLS() {
			return fmt.Errorf("%s: TLS is not supported yet", modName)
		}
		l, err := net.Listen(endp.Network(), endp.Address())
		if err != nil {
			return fmt.Errorf("%s: %v", modName, err)
		}

		e.listenersWg.Add(1)
		go func() {
			e.logger.Println("listening on", endp.String())
			err := e.serv.Serve(l)
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				e.logger.Error("serve failed", err, "endpoint", a)
			}
			e.listenersWg.Done()
		}()
	}

	return nil
}

func (e *Endpoint) Name() string {
	return modName
}

func (e *Endpoint) InstanceName() string {
	return ""
}

func (e *Endpoint) Close() error {
	if err := e.serv.Close(); err != nil {
		return err
	}
	e.listenersWg.Wait()
	return nil
}

func init() {
	module.RegisterEndpoint(modName, New)
}
