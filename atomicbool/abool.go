// Package atomicbool provides a simple wrapper for
// atomic bool for convenience
package atomicbool

import (
	"sync/atomic"
)

// AtomicBool is an atomic bool.
//
// Note: It is not safe to directly copy it.
// Use an AtomicBool.Copy for it.
type AtomicBool uint32

func (b *AtomicBool) Copy() AtomicBool {
	return AtomicBool(atomic.LoadUint32((*uint32)(b)))
}

func (b *AtomicBool) IsSet() bool {
	return atomic.LoadUint32((*uint32)(b)) == 1
}

func (b *AtomicBool) Set(val bool) {
	var i uint32
	if val {
		i = 1
	}
	atomic.StoreUint32((*uint32)(b), i)
}
