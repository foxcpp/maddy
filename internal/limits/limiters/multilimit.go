package limiters

import "context"

// MultiLimit wraps multiple L implementations into a single one, locking them
// in the specified order.
//
// It does not implement any deadlock detection or avoidance algorithms.
type MultiLimit struct {
	Wrapped []L
}

func (ml *MultiLimit) Take() bool {
	for i := 0; i < len(ml.Wrapped); i++ {
		if !ml.Wrapped[i].Take() {
			// Acquire failed, undo acquire for all other resources we already
			// got.
			for _, l := range ml.Wrapped[:i] {
				l.Release()
			}
			return false
		}
	}
	return true
}

func (ml *MultiLimit) TakeContext(ctx context.Context) error {
	for i := 0; i < len(ml.Wrapped); i++ {
		if err := ml.Wrapped[i].TakeContext(ctx); err != nil {
			// Acquire failed, undo acquire for all other resources we already
			// got.
			for _, l := range ml.Wrapped[:i] {
				l.Release()
			}
			return err
		}
	}
	return nil
}

func (ml *MultiLimit) Release() {
	for _, l := range ml.Wrapped {
		l.Release()
	}
}

func (ml *MultiLimit) Close() {
	for _, l := range ml.Wrapped {
		l.Close()
	}
}
