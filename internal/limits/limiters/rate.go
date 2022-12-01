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
	"sync"
	"time"

	"golang.org/x/time/rate"
)

var ErrClosed = errors.New("limiters: Rate bucket is closed")

// Rate structure uses golang.org/x/time/rate rate-limiter for requests using a token
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
	mu      sync.Mutex
	limiter *rate.Limiter
	closed  bool
}

func NewRate(burstSize int, interval time.Duration) *Rate {
	limit := rate.Every(interval)
	if burstSize == 0 {
		limit = rate.Inf
	}
	return &Rate{
		limiter: rate.NewLimiter(limit, burstSize),
		closed:  false,
	}
}

func (r *Rate) Take() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return false
	}

	if err := r.limiter.Wait(context.Background()); err != nil {
		// not changing the L interface of Take method and returning false in case of error
		return false
	}
	return true
}

func (r *Rate) TakeContext(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return ErrClosed
	}

	return r.limiter.WaitN(ctx, 1)
}

func (r Rate) Release() {
}

func (r *Rate) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
}
