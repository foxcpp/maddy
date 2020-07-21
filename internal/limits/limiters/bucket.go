/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package limiters

import (
	"context"
	"sync"
	"time"
)

// BucketSet combines a group of Ls into a single key-indexed structure.
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
	// New function is used to construct underlying L instances.
	//
	// It is safe to change it only when BucketSet is not used by any
	// goroutine.
	New func() L

	// Time after which bucket is considered stale and can be removed from the
	// set. For safe use with Rate limiter, it should be at least as twice as
	// big as Rate refill interval.
	ReapInterval time.Duration

	MaxBuckets int

	mLck sync.Mutex
	m    map[string]*struct {
		r       L
		lastUse time.Time
	}
}

func NewBucketSet(new_ func() L, reapInterval time.Duration, maxBuckets int) *BucketSet {
	return &BucketSet{
		New:          new_,
		ReapInterval: reapInterval,
		MaxBuckets:   maxBuckets,
		m: map[string]*struct {
			r       L
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

func (r *BucketSet) take(key string) L {
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
			r       L
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

func (r *BucketSet) Release(key string) {
	if r.New == nil {
		return
	}

	r.mLck.Lock()
	defer r.mLck.Unlock()

	bucket, ok := r.m[key]
	if !ok {
		return
	}
	bucket.r.Release()
}

func (r *BucketSet) TakeContext(ctx context.Context, key string) error {
	if r.New == nil {
		return nil
	}

	bucket := r.take(key)
	return bucket.TakeContext(ctx)
}
