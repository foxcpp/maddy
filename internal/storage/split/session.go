package split

import (
	"context"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/foxcpp/maddy/framework/module/storage"
)

type deferredSession struct {
	s    *split
	conn *imapserver.Conn
	storage.Session
}

var errAuthenticateFirst = &imap.Error{
	Type: imap.StatusResponseTypeBad,
	Code: imap.ResponseCodeClientBug,
	Text: "Authenticate first",
}

func (s *deferredSession) Login(username, password string) error {
	panic("Login call for storage.Session is no-op")
}

func (s *deferredSession) SetAccount(ctx context.Context, accountID string) error {
	if s.Session != nil {
		if err := s.Session.Close(); err != nil {
			return err
		}
	}

	back, err := s.s.storeForAccount(ctx, accountID)
	if err != nil {
		return err
	}

	sess, data, err := back.NewSession(ctx, s.conn)
	if err != nil {
		return err
	}
	if data.PreAuth {
		panic("PreAuth cannot be used by storage backend directly")
	}

	if err := sess.SetAccount(ctx, accountID); err != nil {
		_ = sess.Close()
		return err
	}

	s.Session = sess
	return nil
}

func (s *deferredSession) Move(w *imapserver.MoveWriter, numSet imap.NumSet, dest string) error {
	if s.Session == nil {
		return errAuthenticateFirst
	}
	return s.Session.(imapserver.SessionMove).Move(w, numSet, dest)
}

func (s *deferredSession) AppendLimit() uint32 {
	if s.Session == nil {
		panic(errAuthenticateFirst)
	}
	return s.Session.(imapserver.SessionAppendLimit).AppendLimit()
}

func (s *deferredSession) Namespace() (*imap.NamespaceData, error) {
	if s.Session == nil {
		return nil, errAuthenticateFirst
	}
	return s.Session.(imapserver.SessionNamespace).Namespace()
}

func (s *deferredSession) Unauthenticate() error {
	if s.Session == nil {
		return nil
	}
	if err := s.Session.Close(); err != nil {
		return err
	}
	s.Session = nil
	return nil
}

func (s *deferredSession) Close() error {
	if s.Session == nil {
		return nil
	}

	return s.Session.Close()
}
