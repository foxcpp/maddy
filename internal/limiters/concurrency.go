package limiters

import "context"

// Semaphore is a convenience wrapper for a channel that implements
// semaphore-kind synchronization.
//
// If the argument given to the NewSemaphore is negative or zero,
// all methods are no-op.
type Semaphore struct {
	c chan struct{}
}

func NewSemaphore(max int) Semaphore {
	return Semaphore{c: make(chan struct{}, max)}
}

func (s Semaphore) Take() {
	if cap(s.c) <= 0 {
		return
	}
	s.c <- struct{}{}
}

func (s Semaphore) TakeContext(ctx context.Context) error {
	if cap(s.c) <= 0 {
		return nil
	}
	select {
	case s.c <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s Semaphore) Release() {
	if cap(s.c) <= 0 {
		return
	}
	select {
	case <-s.c:
	default:
		panic("limiters: mismatched Release call")
	}
}
