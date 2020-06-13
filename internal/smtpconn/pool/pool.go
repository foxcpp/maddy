package pool

import (
	"context"
	"sync"
	"time"
)

type Conn interface {
	Usable() bool
	Close() error
}

type Config struct {
	New                 func(ctx context.Context, key string) (Conn, error)
	MaxKeys             int
	MaxConnsPerKey      int
	MaxConnLifetimeSec  int64
	StaleKeyLifetimeSec int64
}

type slot struct {
	c chan Conn
	// To keep slot size smaller it is just a unix timestamp.
	lastUse int64
}

type P struct {
	cfg      Config
	keys     map[string]slot
	keysLock sync.Mutex
}

func New(cfg Config) *P {
	if cfg.New == nil {
		cfg.New = func(context.Context, string) (Conn, error) {
			return nil, nil
		}
	}

	return &P{
		cfg:  cfg,
		keys: make(map[string]slot, cfg.MaxKeys),
	}
}

func (p *P) Get(ctx context.Context, key string) (Conn, error) {
	// TODO: See if it is possible to get rid of this lock.
	p.keysLock.Lock()
	defer p.keysLock.Unlock()

	bucket, ok := p.keys[key]
	if !ok {
		return p.cfg.New(ctx, key)
	}

	if time.Now().Unix()-bucket.lastUse > p.cfg.MaxConnLifetimeSec {
		// Drop bucket.
		close(bucket.c)
		for conn := range bucket.c {
			conn.Close()
		}
		delete(p.keys, key)

		return p.cfg.New(ctx, key)
	}

	for {
		var conn Conn
		select {
		case conn, ok = <-bucket.c:
			if !ok {
				return p.cfg.New(ctx, key)
			}
		default:
			return p.cfg.New(ctx, key)
		}

		if !conn.Usable() {
			continue
		}

		return conn, nil
	}
}

func (p *P) Return(key string, c Conn) {
	p.keysLock.Lock()
	defer p.keysLock.Unlock()

	if p.keys == nil {
		return
	}

	bucket, ok := p.keys[key]
	if !ok {
		// Garbage-collect stale buckets.
		if len(p.keys) == p.cfg.MaxKeys {
			for k, v := range p.keys {
				if v.lastUse+p.cfg.StaleKeyLifetimeSec > time.Now().Unix() {
					continue
				}

				close(v.c)
				for conn := range v.c {
					conn.Close()
				}
				delete(p.keys, k)
			}
		}

		bucket = slot{
			c:       make(chan Conn, p.cfg.MaxConnsPerKey),
			lastUse: time.Now().Unix(),
		}
		p.keys[key] = bucket
	}

	select {
	case bucket.c <- c:
		bucket.lastUse = time.Now().Unix()
	default:
		// Let it go, let it go...
		c.Close()
	}
}

func (p *P) Close() {
	p.keysLock.Lock()
	defer p.keysLock.Unlock()

	for k, v := range p.keys {
		close(v.c)
		for conn := range v.c {
			conn.Close()
		}
		delete(p.keys, k)
	}
	p.keys = nil
}
