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
	"errors"
	"fmt"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-sasl"
	"github.com/foxcpp/maddy/framework/module/storage"
	"github.com/foxcpp/maddy/internal/auth"
)

func (endp *Endpoint) NewSession(conn *imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
	storageSession, data, err := endp.Store.NewSession(context.TODO(), conn)
	if err != nil {
		return nil, nil, err
	}

	return &session{
		Session: storageSession,
		conn:    conn,
		endp:    endp,
	}, data, nil
}

var _ storage.Session = (*session)(nil)
var _ imapserver.SessionAppendLimit = (*session)(nil)
var _ imapserver.SessionMove = (*session)(nil)
var _ imapserver.SessionNamespace = (*session)(nil)

type session struct {
	storage.Session
	conn *imapserver.Conn
	endp *Endpoint

	authUsername string
}

func (s *session) Login(username, password string) error {
	// saslAuth handles AuthMap calling.
	err := s.endp.saslAuth.AuthPlain(username, password)
	if err != nil {
		s.endp.log.Error("authentication failed", err,
			"username", username, "src_ip", s.conn.NetConn().RemoteAddr())
		return imapserver.ErrAuthFailed
	}

	storageUsername, err := s.endp.usernameForStorage(context.TODO(), username)
	if err != nil {
		if errors.Is(err, imapserver.ErrAuthFailed) {
			return err
		}
		s.endp.log.Error("authentication failed due to an internal error", err,
			"username", username, "src_ip", s.conn.NetConn().RemoteAddr())
		return fmt.Errorf("internal server error")
	}

	s.authUsername = username
	return s.SetAccount(context.TODO(), storageUsername)
}

func (s *session) AuthenticateMechanisms() []string {
	return s.endp.saslAuth.SASLMechanisms()
}

func (s *session) Authenticate(mech string) (sasl.Server, error) {
	srv := s.endp.saslAuth.CreateSASL(
		mech, s.conn.NetConn().RemoteAddr(),
		func(identity string, data auth.ContextData) error {
			return s.SetAccount(context.TODO(), identity)
		},
	)
	return srv, nil
}

func (s *session) Unauthenticate() error {
	s.authUsername = ""
	return s.UnsetAccountID(context.TODO())
}

func (s *session) AppendLimit() uint32 {
	return s.Session.(imapserver.SessionAppendLimit).AppendLimit()
}

func (s *session) Move(w *imapserver.MoveWriter, numSet imap.NumSet, dest string) error {
	return s.Session.(imapserver.SessionMove).Move(w, numSet, dest)
}

func (s *session) Namespace() (*imap.NamespaceData, error) {
	return s.Session.(imapserver.SessionNamespace).Namespace()
}
