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
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/emersion/go-sasl"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/auth/sasllogin"
	"github.com/foxcpp/maddy/internal/authz"
)

var (
	ErrUnsupportedMech = errors.New("Unsupported SASL mechanism")
	ErrInvalidAuthCred = errors.New("auth: invalid credentials")
)

// SASLAuth is a wrapper that initializes sasl.Server using authenticators that
// call maddy module objects.
//
// It also handles username translation using auth_map and auth_map_normalize
// (AuthMap and AuthMapNormalize should be set).
//
// It supports reporting of multiple authorization identities so multiple
// accounts can be associated with a single set of credentials.
type SASLAuth struct {
	Log         log.Logger
	OnlyFirstID bool
	EnableLogin bool

	AuthMap       module.Table
	AuthNormalize authz.NormalizeFunc

	Plain []module.PlainAuth
}

func (s *SASLAuth) SASLMechanisms() []string {
	var mechs []string

	if len(s.Plain) != 0 {
		mechs = append(mechs, sasl.Plain)
		if s.OnlyFirstID {
			mechs = append(mechs, sasl.Login)
		}
	}

	return mechs
}

func (s *SASLAuth) usernameForAuth(ctx context.Context, saslUsername string) (string, error) {
	if s.AuthNormalize != nil {
		var err error
		saslUsername, err = s.AuthNormalize(saslUsername)
		if err != nil {
			return "", err
		}
	}

	if s.AuthMap == nil {
		return saslUsername, nil
	}

	mapped, ok, err := s.AuthMap.Lookup(ctx, saslUsername)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrInvalidAuthCred
	}

	if saslUsername != mapped {
		s.Log.DebugMsg("using mapped username for authentication", "username", saslUsername, "mapped_username", mapped)
	}

	return mapped, nil
}

func (s *SASLAuth) AuthPlain(username, password string) error {
	if len(s.Plain) == 0 {
		return ErrUnsupportedMech
	}

	var lastErr error
	for _, p := range s.Plain {
		username, err := s.usernameForAuth(context.TODO(), username)
		if err != nil {
			return err
		}

		lastErr = p.AuthPlain(username, password)
		if lastErr == nil {
			return nil
		}
	}

	return fmt.Errorf("no auth. provider accepted creds, last err: %w", lastErr)
}

type ContextData struct {
	// Authentication username. May be different from identity.
	Username string

	// Password used for password-based mechanisms.
	Password string
}

// CreateSASL creates the sasl.Server instance for the corresponding mechanism.
func (s *SASLAuth) CreateSASL(mech string, remoteAddr net.Addr, successCb func(identity string, data ContextData) error) sasl.Server {
	switch mech {
	case sasl.Plain:
		return sasl.NewPlainServer(func(identity, username, password string) error {
			if identity == "" {
				identity = username
			}
			if identity != username {
				return ErrInvalidAuthCred
			}

			username, err := s.usernameForAuth(context.Background(), username)
			if err != nil {
				return err
			}

			err = s.AuthPlain(username, password)
			if err != nil {
				s.Log.Error("authentication failed", err, "username", username, "src_ip", remoteAddr)
				return ErrInvalidAuthCred
			}

			return successCb(identity, ContextData{
				Username: username,
				Password: password,
			})
		})
	case sasl.Login:
		if !s.EnableLogin {
			return FailingSASLServ{Err: ErrUnsupportedMech}
		}

		return sasllogin.NewLoginServer(func(username, password string) error {
			username, err := s.usernameForAuth(context.Background(), username)
			if err != nil {
				return err
			}

			err := s.AuthPlain(username, password)
			if err != nil {
				s.Log.Error("authentication failed", err, "username", username, "src_ip", remoteAddr)
				return ErrInvalidAuthCred
			}

			return successCb(username, ContextData{
				Username: username,
				Password: password,
			})
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
