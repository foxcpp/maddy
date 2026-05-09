package storage

import (
	"context"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
)

type Session interface {
	imapserver.Session
	imapserver.SessionUnauthenticate
	SetAccount(ctx context.Context, accountID string) error
	UnsetAccountID(ctx context.Context) error
}

type Storage interface {
	NewSession(ctx context.Context, conn *imapserver.Conn) (Session, *imapserver.GreetingData, error)
	AvailableCaps() imap.CapSet
}
