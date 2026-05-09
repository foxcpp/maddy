package imapsql2

import (
	"context"
	"errors"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/foxcpp/maddy-storage/pkg/managed"
	"github.com/foxcpp/maddy/framework/module/storage"
)

func (store *Storage) NewSession(ctx context.Context, conn *imapserver.Conn) (storage.Session, *imapserver.GreetingData, error) {
	imapSession, greetingData, err := store.imapOpts.NewSession(conn)
	if err != nil {
		return nil, nil, err
	}
	maddySession, ok := imapSession.(storage.Session)
	if !ok {
		panic("imap session does not implement storage.Session")
	}
	return maddySession, greetingData, nil
}

func (store *Storage) AvailableCaps() imap.CapSet {
	return store.imapOpts.Caps
}

func (store *Storage) Lookup(ctx context.Context, s string) (string, bool, error) {
	acct, err := store.manage.GetAccountInfo(ctx, s)
	if err != nil {
		if errors.Is(err, managed.ErrAccountNotFound) {
			return "", false, nil
		}
		return "", false, err
	}

	return "", true, nil
}
