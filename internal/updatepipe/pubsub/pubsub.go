package pubsub

import "context"

type PubSub interface {
	Subscribe(ctx context.Context, key string) error
	Unsubscribe(ctx context.Context, key string) error
	Publish(key, payload string) error
	Listener() chan Msg
	Close() error
}
