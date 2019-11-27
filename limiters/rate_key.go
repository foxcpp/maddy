package limiters

import (
	"context"
	"sync"
	"time"
)

// RateSet combines a group of token bucket as implemented by the Rate
// structure into a single key-indexes structure. Basically, each unique key
// gets its own counter. The main use case for RateSet is to apply per-resource
// rate limiting.
//
// Amount of buckets is limited to a certain value. When the size of internal
// map is around or equal to that value, next Take call will attempt to remove
// any stale buckets from the group. If it is not possible to do so (all
// buckets are in active use), Take will return false. Alternatively, in some
// rare cases, some other (undefined) waiting Take can return false.
//
// Similarly to Rate, if burst = 0, all methods are no-op and always succeed.
type RateSet struct {
	maxBuckets int
	interval   time.Duration
	burst      int

	mLck sync.Mutex
	m    map[string]*struct {
		r       Rate
		lastUse time.Time
	}
}

func NewRateSet(burst int, interval time.Duration, maxBuckets int) *RateSet {
	return &RateSet{
		maxBuckets: maxBuckets,
		interval:   interval,
		burst:      burst,
		m: map[string]*struct {
			r       Rate
			lastUse time.Time
		}{},
	}
}

func (r *RateSet) Close() {
	r.mLck.Lock()
	defer r.mLck.Unlock()

	for _, v := range r.m {
		v.r.Close()
	}
}

func (r *RateSet) take(key string) *Rate {
	r.mLck.Lock()
	defer r.mLck.Unlock()

	if len(r.m) > r.maxBuckets {
		now := time.Now()
		// Attempt to get rid of stale buckets.
		for k, v := range r.m {
			// Skip non-full buckets, they are likely in use.
			if len(v.r.bucket) != r.burst {
				continue
			}
			if v.lastUse.Sub(now) > r.interval*2 {
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
		if len(r.m) > r.maxBuckets {
			return nil
		}
	}

	bucket, ok := r.m[key]
	if !ok {
		r.m[key] = &struct {
			r       Rate
			lastUse time.Time
		}{
			r:       NewRate(r.burst, r.interval),
			lastUse: time.Now(),
		}
		bucket = r.m[key]
	}
	r.m[key].lastUse = time.Now()

	return &bucket.r
}

func (r *RateSet) Take(key string) bool {
	if r.burst == 0 {
		return true
	}

	bucket := r.take(key)
	return bucket.Take()
}

func (r *RateSet) TakeContext(ctx context.Context, key string) error {
	if r.burst == 0 {
		return nil
	}

	bucket := r.take(key)
	return bucket.TakeContext(ctx)
}
