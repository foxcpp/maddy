package resource

import (
	"sync"
)

// Tracker is a container wrapper that tracks whether resources were used since
// last MarkAllUnused call.
type Tracker[T Resource] struct {
	C Container[T]

	usedLock sync.Mutex
	used     map[string]bool
}

func NewTracker[T Resource](c Container[T]) *Tracker[T] {
	return &Tracker[T]{C: c, used: make(map[string]bool)}
}

func (t *Tracker[T]) Close() error {
	return t.C.Close()
}

func (t *Tracker[T]) MarkAllUnused() {
	t.usedLock.Lock()
	defer t.usedLock.Unlock()

	t.used = make(map[string]bool)
}

func (t *Tracker[T]) GetOpen(key string, open func() (T, error)) (T, error) {
	t.usedLock.Lock()
	t.used[key] = true
	t.usedLock.Unlock()

	return t.C.GetOpen(key, open)
}

func (t *Tracker[T]) CloseUnused(isUsed func(key string) bool) error {
	t.usedLock.Lock()
	defer t.usedLock.Unlock()

	return t.C.CloseUnused(func(key string) bool {
		used := t.used[key]
		used = used && isUsed(key)
		if !used {
			delete(t.used, key)
		}
		return used
	})
}
