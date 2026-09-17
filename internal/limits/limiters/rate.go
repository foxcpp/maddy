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
	"time"

	"golang.org/x/time/rate"
)

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
	limiter *rate.Limiter
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewRate(burstSize int, interval time.Duration) Rate {
	ctx, cancel := context.WithCancel(context.TODO())
	r := Rate{limiter: nil, ctx: ctx, cancel: cancel}
	if burstSize > 0 {
		r.limiter = rate.NewLimiter(rate.Every(interval), burstSize)
	}
	return r
}

func (r Rate) Take() bool {
	if r.limiter == nil {
		return true
	}

	if err := r.limiter.Wait(r.ctx); err != nil {
		return false
	}
	return true
}

func (r Rate) TakeContext(ctx context.Context) error {
	if r.limiter == nil {
		return nil
	}
	select {
	case <-r.ctx.Done():
		return ErrClosed
	default:
	}
	reqCtx, reqCancel := context.WithCancel(ctx)
	defer reqCancel()

	stop := context.AfterFunc(r.ctx, func() {
		reqCancel()
	})
	defer stop()

	return r.limiter.Wait(reqCtx)
}

func (r Rate) Release() {
}

func (r Rate) Close() {
	r.cancel()
}
