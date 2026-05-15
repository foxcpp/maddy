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

package module

import (
	"crypto/tls"
	"errors"
	"net"
)

// ErrUnknownCredentials should be returned by auth. provider if supplied
// credentials are valid for it but are not recognized (e.g. not found in
// used DB).
var ErrUnknownCredentials = errors.New("unknown credentials")

type ProxiedTLSContext struct {
	ValidCert    bool
	CertUsername string
	Cipher       string
	CipherBits   int
	PFS          string
	Version      uint16
}

type AuthContext struct {
	Service    string
	LocalAddr  net.Addr
	RemoteAddr net.Addr
	TLS        *tls.ConnectionState
	ProxiedTLS *ProxiedTLSContext // populated instead of TLS if TLS is terminated by upstream and TLS info is available
}

type BearerTokenError struct {
	Err           error
	Status        string
	Schemes       string // probably, "bearer"
	Scope         string
	OIDCConfigURL string
}

func (e BearerTokenError) Error() string { return e.Err.Error() }
func (e BearerTokenError) Unwrap() error { return e.Err }

type BearerTokenAuth interface {
	AuthBearerToken(ctx *AuthContext, username, token string) (identity string, err error)
}

type ExternalAuth interface {
	AuthExternal(ctx *AuthContext, requestedIdentity string) (finalIdentity string, err error)
}

// PlainAuth is the interface implemented by modules providing authentication using
// username:password pairs.
//
// Modules implementing this interface should be registered with "auth." prefix in name.
type PlainAuth interface {
	AuthPlain(ctx *AuthContext, username, password string) error
}

// PlainUserDB is a local credentials store that can be managed using maddy command
// utility.
type PlainUserDB interface {
	PlainAuth
	ListUsers() ([]string, error)
	CreateUser(username, password string) error
	SetUserPassword(username, password string) error
	DeleteUser(username string) error
}
