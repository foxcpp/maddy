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

func (s Semaphore) Take() bool {
	if cap(s.c) <= 0 {
		return true
	}
	s.c <- struct{}{}
	return true
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

func (s Semaphore) Close() {
}
