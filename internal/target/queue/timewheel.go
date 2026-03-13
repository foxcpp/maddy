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

package queue

import (
	"container/list"
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type TimeSlot[Value any] struct {
	Time  time.Time
	Value Value
}

type TimeWheel[Value any] struct {
	stopped uint32

	slots     *list.List
	slotsLock sync.Mutex

	updateNotify chan time.Time
	stopNotify   chan struct{}
	tickerCtx    context.Context
	tickerCancel context.CancelFunc

	dispatch func(context.Context, TimeSlot[Value])
}

func NewTimeWheel[Value any](dispatch func(context.Context, TimeSlot[Value])) *TimeWheel[Value] {
	ctx, cancel := context.WithCancel(context.Background())

	tw := &TimeWheel[Value]{
		slots:        list.New(),
		stopNotify:   make(chan struct{}),
		tickerCtx:    ctx,
		tickerCancel: cancel,
		updateNotify: make(chan time.Time),
		dispatch:     dispatch,
	}
	go tw.tick(context.Background())
	return tw
}

func (tw *TimeWheel[Value]) Add(target time.Time, value Value) {
	if atomic.LoadUint32(&tw.stopped) == 1 {
		// Already stopped, ignore.
		return
	}

	tw.slotsLock.Lock()
	tw.slots.PushBack(TimeSlot[Value]{Time: target, Value: value})
	tw.slotsLock.Unlock()

	tw.updateNotify <- target
}

func (tw *TimeWheel[Value]) Close() {
	atomic.StoreUint32(&tw.stopped, 1)

	// Idempotent Close is convenient sometimes.
	if tw.stopNotify == nil {
		return
	}

	tw.tickerCancel()

	tw.stopNotify <- struct{}{}
	<-tw.stopNotify

	tw.stopNotify = nil

	close(tw.updateNotify)
}

func (tw *TimeWheel[Value]) tick(ctx context.Context) {
	for {
		now := time.Now()
		// Look for list element closest to now.
		tw.slotsLock.Lock()
		var closestSlot TimeSlot[Value]
		var closestEl *list.Element
		for e := tw.slots.Front(); e != nil; e = e.Next() {
			slot := e.Value.(TimeSlot[Value])
			if slot.Time.Sub(now) < closestSlot.Time.Sub(now) || closestEl == nil {
				closestSlot = slot
				closestEl = e
			}
		}
		tw.slotsLock.Unlock()
		// Only this goroutine removes elements from TimeWheel so we can be safe using closestSlot.

		// Queue is empty. Just wait until update.
		if closestEl == nil {
			select {
			case <-tw.updateNotify:
				continue
			case <-tw.stopNotify:
				tw.stopNotify <- struct{}{}
				return
			}
		}

		timer := time.NewTimer(closestSlot.Time.Sub(now))

	selectloop:
		for {
			select {
			case <-timer.C:
				tw.slotsLock.Lock()
				tw.slots.Remove(closestEl)
				tw.slotsLock.Unlock()

				tw.dispatch(ctx, closestSlot)

				break selectloop
			case newTarget := <-tw.updateNotify:
				// Avoid unnecessary restarts if new target is not going to affect our
				// current wait time.
				if closestSlot.Time.Sub(now) <= newTarget.Sub(now) {
					continue
				}

				timer.Stop()
				// Recalculate new slot time.
				break selectloop
			case <-tw.stopNotify:
				tw.stopNotify <- struct{}{}
				return
			}
		}
	}
}
