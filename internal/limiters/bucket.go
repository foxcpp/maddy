package limiters

import (
	"context"
	"sync"
	"time"
)

// BucketSet combines a group of Limiters into a single key-indexed structure.
// Basically, each unique key gets its own counter. The main use case for
// BucketSet is to apply per-resource rate limiting.
//
// Amount of buckets is limited to a certain value. When the size of internal
// map is around or equal to that value, next Take call will attempt to remove
// any stale buckets from the group. If it is not possible to do so (all
// buckets are in active use), Take will return false. Alternatively, in some
// rare cases, some other (undefined) waiting Take can return false.
//
// A BucksetSet without a New function assigned is no-op: Take and TakeContext
// always succeed and Release does nothing.
type BucketSet struct {
	// New function is used to construct underlying Limiter instances.
	//
	// It is safe to change it only when BucketSet is not used by any
	// goroutine.
	New func() Limiter

	// Time after which bucket is considered stale and can be removed from the
	// set. For safe use with Rate limiter, it should be at least as twice as
	// big as Rate refill interval.
	ReapInterval time.Duration

	MaxBuckets int

	mLck sync.Mutex
	m    map[string]*struct {
		r       Limiter
		lastUse time.Time
	}
}

func NewBucketSet(new_ func() Limiter, reapInterval time.Duration, maxBuckets int) *BucketSet {
	return &BucketSet{
		New:          new_,
		ReapInterval: reapInterval,
		MaxBuckets:   maxBuckets,
		m: map[string]*struct {
			r       Limiter
			lastUse time.Time
		}{},
	}
}

func (r *BucketSet) Close() {
	r.mLck.Lock()
	defer r.mLck.Unlock()

	for _, v := range r.m {
		v.r.Close()
	}
}

func (r *BucketSet) take(key string) Limiter {
	r.mLck.Lock()
	defer r.mLck.Unlock()

	if len(r.m) > r.MaxBuckets {
		now := time.Now()
		// Attempt to get rid of stale buckets.
		for k, v := range r.m {
			if v.lastUse.Sub(now) > r.ReapInterval {
				// Drop the bucket, if there happen to be any waiting Take for it.
				// It will return 'false', but this is fine for us since this
				// whole 'reaping' process will run only when we are under a
				// high load and dropping random requests in this case is a
				// more or less reasonable thing to do.
				v.r.Close()
				delete(r.m, k)
			}
		}

		// Still full? E.g. all buckets are in use.
		if len(r.m) > r.MaxBuckets {
			return nil
		}
	}

	bucket, ok := r.m[key]
	if !ok {
		r.m[key] = &struct {
			r       Limiter
			lastUse time.Time
		}{
			r:       r.New(),
			lastUse: time.Now(),
		}
		bucket = r.m[key]
	}
	r.m[key].lastUse = time.Now()

	return bucket.r
}

func (r *BucketSet) Take(key string) bool {
	if r.New == nil {
		return true
	}

	bucket := r.take(key)
	return bucket.Take()
}

func (r *BucketSet) TakeContext(ctx context.Context, key string) error {
	if r.New == nil {
		return nil
	}

	bucket := r.take(key)
	return bucket.TakeContext(ctx)
}
