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

package dovecotsasl

import (
	"fmt"
	"net"

	"github.com/emersion/go-sasl"
	dovecotsasl "github.com/foxcpp/go-dovecot-sasl"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/auth"
)

type Auth struct {
	instName       string
	serverEndpoint string
	log            log.Logger

	network string
	addr    string

	mechanisms map[string]dovecotsasl.Mechanism
}

const modName = "dovecot_sasl"

func New(_, instName string, _, inlineArgs []string) (module.Module, error) {
	a := &Auth{
		instName: instName,
		log:      log.Logger{Name: modName, Debug: log.DefaultLogger.Debug},
	}

	switch len(inlineArgs) {
	case 0:
	case 1:
		a.serverEndpoint = inlineArgs[0]
	default:
		return nil, fmt.Errorf("%s: one or none arguments needed", modName)
	}

	return a, nil
}

func (a *Auth) Name() string {
	return modName
}

func (a *Auth) InstanceName() string {
	return a.instName
}

func (a *Auth) getConn() (*dovecotsasl.Client, error) {
	// TODO: Connection pooling
	conn, err := net.Dial(a.network, a.addr)
	if err != nil {
		return nil, fmt.Errorf("%s: unable to contact server: %v", modName, err)
	}

	cl, err := dovecotsasl.NewClient(conn)
	if err != nil {
		return nil, fmt.Errorf("%s: unable to contact server: %v", modName, err)
	}

	return cl, nil
}

func (a *Auth) returnConn(cl *dovecotsasl.Client) {
	cl.Close()
}

func (a *Auth) Init(cfg *config.Map) error {
	cfg.String("endpoint", false, false, a.serverEndpoint, &a.serverEndpoint)
	if _, err := cfg.Process(); err != nil {
		return err
	}
	if a.serverEndpoint == "" {
		return fmt.Errorf("%s: missing server endpoint", modName)
	}

	endp, err := config.ParseEndpoint(a.serverEndpoint)
	if err != nil {
		return fmt.Errorf("%s: invalid server endpoint: %v", modName, err)
	}

	// Dial once to check usability and also to get list of mechanisms.
	conn, err := net.Dial(endp.Scheme, endp.Address())
	if err != nil {
		return fmt.Errorf("%s: unable to contact server: %v", modName, err)
	}

	cl, err := dovecotsasl.NewClient(conn)
	if err != nil {
		return fmt.Errorf("%s: unable to contact server: %v", modName, err)
	}

	defer cl.Close()
	a.mechanisms = make(map[string]dovecotsasl.Mechanism, len(cl.ConnInfo().Mechs))
	for name, mech := range cl.ConnInfo().Mechs {
		if mech.Private {
			continue
		}
		a.mechanisms[name] = mech
	}

	a.network = endp.Scheme
	a.addr = endp.Address()

	return nil
}

func (a *Auth) AuthPlain(username, password string) error {
	if _, ok := a.mechanisms[sasl.Plain]; ok {
		cl, err := a.getConn()
		if err != nil {
			return exterrors.WithTemporary(err, true)
		}
		defer a.returnConn(cl)

		// Pretend it is SMTPS even though we really don't know.
		// We also have no connection information to pass to the server...
		return cl.Do("SMTP", sasl.NewPlainClient("", username, password),
			dovecotsasl.Secured, dovecotsasl.NoPenalty)
	}
	if _, ok := a.mechanisms[sasl.Login]; ok {
		cl, err := a.getConn()
		if err != nil {
			return err
		}
		defer a.returnConn(cl)

		return cl.Do("SMTP", sasl.NewLoginClient(username, password),
			dovecotsasl.Secured, dovecotsasl.NoPenalty)
	}

	return auth.ErrUnsupportedMech
}

func init() {
	module.Register(modName, New)
}
