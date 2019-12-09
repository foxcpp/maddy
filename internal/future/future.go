package future

import (
	"context"
	"sync"
)

// The Future object implements a container for (value, error) pair that "will
// be populated later" and allows multiple users to wait for it to be set.
//
// It should not be copied after first use.
type Future struct {
	mu  sync.RWMutex
	set bool
	val interface{}
	err error

	notify chan struct{}
}

func New() *Future {
	return &Future{notify: make(chan struct{})}
}

// Set sets the Future (value, error) pair. All currently blocked and future
// Get calls will return it.
func (f *Future) Set(val interface{}, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.set {
		panic("Future.Set called multiple times")
	}

	f.set = true
	f.val = val
	f.err = err

	close(f.notify)
}

func (f *Future) Get() (interface{}, error) {
	return f.GetContext(context.Background())
}

func (f *Future) GetContext(ctx context.Context) (interface{}, error) {
	f.mu.RLock()
	if f.set {
		val := f.val
		err := f.err
		f.mu.RUnlock()
		return val, err
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

	return f.val, f.err
}
