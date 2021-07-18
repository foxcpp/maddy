package pubsub

import (
	"context"
	"database/sql"
	"time"

	"github.com/foxcpp/maddy/framework/log"
	"github.com/lib/pq"
)

type Msg struct {
	Key     string
	Payload string
}

type PqPubSub struct {
	Notify chan Msg

	L      *pq.Listener
	sender *sql.DB

	Log log.Logger
}

func NewPQ(dsn string) (*PqPubSub, error) {
	l := &PqPubSub{
		Log:    log.Logger{Name: "pgpubsub"},
		Notify: make(chan Msg),
	}
	l.L = pq.NewListener(dsn, 10*time.Second, time.Minute, l.eventHandler)
	var err error
	l.sender, err = sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	go func() {
		defer close(l.Notify)
		for n := range l.L.Notify {
			if n == nil {
				continue
			}

			l.Notify <- Msg{Key: n.Channel, Payload: n.Extra}
		}
	}()

	return l, nil
}

func (l *PqPubSub) Close() error {
	l.sender.Close()
	l.L.Close()
	return nil
}

func (l *PqPubSub) eventHandler(ev pq.ListenerEventType, err error) {
	switch ev {
	case pq.ListenerEventConnected:
		l.Log.DebugMsg("connected")
	case pq.ListenerEventReconnected:
		l.Log.Msg("connection reestablished")
	case pq.ListenerEventConnectionAttemptFailed:
		l.Log.Error("connection attempt failed", err)
	case pq.ListenerEventDisconnected:
		l.Log.Msg("connection closed", "err", err)
	}
}

func (l *PqPubSub) Subscribe(_ context.Context, key string) error {
	return l.L.Listen(key)
}

func (l *PqPubSub) Unsubscribe(_ context.Context, key string) error {
	return l.L.Unlisten(key)
}

func (l *PqPubSub) Publish(key, payload string) error {
	_, err := l.sender.Exec(`SELECT pg_notify($1, $2)`, key, payload)
	return err
}

func (l *PqPubSub) Listener() chan Msg {
	return l.Notify
}
