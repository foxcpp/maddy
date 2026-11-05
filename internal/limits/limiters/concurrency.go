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

	"golang.org/x/sync/semaphore"
)

// Semaphore is a convenience wrapper for a channel that implements
// semaphore-kind synchronization.
//
// If the argument given to the NewSemaphore is negative or zero,
// all methods are no-op.
type Semaphore struct {
	weighted *semaphore.Weighted
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewSemaphore(max int) Semaphore {
	ctx, cancel := context.WithCancel(context.TODO())
	s := Semaphore{weighted: nil, ctx: ctx, cancel: cancel}
	if max > 0 {
		s.weighted = semaphore.NewWeighted(int64(max))
	}
	return s
}

func (s Semaphore) Take() bool {
	if s.weighted == nil {
		return true
	}

	if err := s.weighted.Acquire(s.ctx, 1); err != nil {
		return false
	}
	return true
}

func (s Semaphore) TakeContext(ctx context.Context) error {
	if s.weighted == nil {
		return nil
	}
	select {
	case <-s.ctx.Done():
		return ErrClosed
	default:
	}
	reqCtx, reqCancel := context.WithCancel(ctx)
	defer reqCancel()

	stop := context.AfterFunc(s.ctx, func() {
		reqCancel()
	})
	defer stop()

	return s.weighted.Acquire(reqCtx, 1)
}

func (s Semaphore) Release() {
	if s.weighted == nil {
		return
	}
	select {
	case <-s.ctx.Done():
		return
	default:
		s.weighted.Release(1)
	}
}

func (s Semaphore) Close() {
	s.cancel()
}
