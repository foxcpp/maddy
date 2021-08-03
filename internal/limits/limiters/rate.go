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
	"errors"
	"time"
)

var ErrClosed = errors.New("limiters: Rate bucket is closed")

// Rate structure implements a basic rate-limiter for requests using the token
// bucket approach.
//
// Take() is expected to be called before each request. Excessive calls will
// block. Timeouts can be implemented using the TakeContext method.
//
// Rate.Close causes all waiting Take to return false. TakeContext returns
// ErrClosed in this case.
//
// If burstSize = 0, all methods are no-op and always succeed.
type Rate struct {
	bucket chan struct{}
	stop   chan struct{}
}

func NewRate(burstSize int, interval time.Duration) Rate {
	r := Rate{
		bucket: make(chan struct{}, burstSize),
		stop:   make(chan struct{}),
	}

	if burstSize == 0 {
		return r
	}

	for i := 0; i < burstSize; i++ {
		r.bucket <- struct{}{}
	}

	go r.fill(burstSize, interval)
	return r
}

func (r Rate) fill(burstSize int, interval time.Duration) {
	t := time.NewTimer(interval)
	defer t.Stop()
	for {
		t.Reset(interval)
		select {
		case <-t.C:
		case <-r.stop:
			close(r.bucket)
			return
		}

	fill:
		for i := 0; i < burstSize; i++ {
			select {
			case r.bucket <- struct{}{}:
			default:
				// If there are no Take pending and the bucket is already
				// full - don't block.
				break fill
			}
		}
	}
}

func (r Rate) Take() bool {
	if cap(r.bucket) == 0 {
		return true
	}

	_, ok := <-r.bucket
	return ok
}

func (r Rate) TakeContext(ctx context.Context) error {
	if cap(r.bucket) == 0 {
		return nil
	}

	select {
	case _, ok := <-r.bucket:
		if !ok {
			return ErrClosed
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r Rate) Release() {
}

func (r Rate) Close() {
	close(r.stop)
}
