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

package auth

import (
	"errors"
	"fmt"
	"net"

	"github.com/emersion/go-sasl"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
)

var (
	ErrUnsupportedMech = errors.New("Unsupported SASL mechanism")
	ErrInvalidAuthCred = errors.New("auth: invalid credentials")
)

// SASLAuth is a wrapper that initializes sasl.Server using authenticators that
// call maddy module objects.
//
// It supports reporting of multiple authorization identities so multiple
// accounts can be associated with a single set of credentials.
type SASLAuth struct {
	Log         log.Logger
	OnlyFirstID bool

	Plain []module.PlainAuth
}

func (s *SASLAuth) SASLMechanisms() []string {
	var mechs []string

	if len(s.Plain) != 0 {
		mechs = append(mechs, sasl.Plain, sasl.Login)
	}

	return mechs
}

func (s *SASLAuth) AuthPlain(username, password string) error {
	if len(s.Plain) == 0 {
		return ErrUnsupportedMech
	}

	var lastErr error
	for _, p := range s.Plain {
		lastErr = p.AuthPlain(username, password)
		if lastErr == nil {
			return nil
		}
	}

	return fmt.Errorf("no auth. provider accepted creds, last err: %w", lastErr)
}

// CreateSASL creates the sasl.Server instance for the corresponding mechanism.
func (s *SASLAuth) CreateSASL(mech string, remoteAddr net.Addr, successCb func(identity string) error) sasl.Server {
	switch mech {
	case sasl.Plain:
		return sasl.NewPlainServer(func(identity, username, password string) error {
			if identity == "" {
				identity = username
			}

			err := s.AuthPlain(username, password)
			if err != nil {
				s.Log.Error("authentication failed", err, "username", username, "src_ip", remoteAddr)
				return ErrInvalidAuthCred
			}

			return successCb(identity)
		})
	case sasl.Login:
		return sasl.NewLoginServer(func(username, password string) error {
			err := s.AuthPlain(username, password)
			if err != nil {
				s.Log.Error("authentication failed", err, "username", username, "src_ip", remoteAddr)
				return ErrInvalidAuthCred
			}

			return successCb(username)
		})
	}
	return FailingSASLServ{Err: ErrUnsupportedMech}
}

// AddProvider adds the SASL authentication provider to its mapping by parsing
// the 'auth' configuration directive.
func (s *SASLAuth) AddProvider(m *config.Map, node config.Node) error {
	var any interface{}
	if err := modconfig.ModuleFromNode("auth", node.Args, node, m.Globals, &any); err != nil {
		return err
	}

	hasAny := false
	if plainAuth, ok := any.(module.PlainAuth); ok {
		s.Plain = append(s.Plain, plainAuth)
		hasAny = true
	}

	if !hasAny {
		return config.NodeErr(node, "auth: specified module does not provide any SASL mechanism")
	}
	return nil
}

type FailingSASLServ struct{ Err error }

func (s FailingSASLServ) Next([]byte) ([]byte, bool, error) {
	return nil, true, s.Err
}
