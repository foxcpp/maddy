package future

import (
	"context"
	"sync"
)

type Future struct {
	mu  sync.RWMutex
	set bool
	val interface{}

	notify chan struct{}
}

func New() *Future {
	return &Future{notify: make(chan struct{})}
}

func (f *Future) Set(val interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.set {
		panic("Future.Set called multiple times")
	}

	f.set = true
	f.val = val

	close(f.notify)
}

func (f *Future) Get() interface{} {
	val, _ := f.GetContext(context.Background())
	return val
}

func (f *Future) GetContext(ctx context.Context) (interface{}, error) {
	f.mu.RLock()
	if f.set {
		val := f.val
		f.mu.RUnlock()
		return val, nil
	}

	f.mu.RUnlock()
	select {
	case <-f.notify:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	f.mu.RLock()
	defer f.mu.RUnlock()
	if !f.set {
		panic("future: Notification received, but value is not set")
	}

	return f.val, nil
}
