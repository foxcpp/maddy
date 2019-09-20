package queue

import (
	"container/list"
	"sync"
	"time"
)

type TimeSlot struct {
	Time  time.Time
	Value interface{}
}

type TimeWheel struct {
	slots     *list.List
	slotsLock sync.Mutex

	updateNotify chan time.Time
	stopNotify   chan struct{}

	dispatch chan TimeSlot
}

func NewTimeWheel() *TimeWheel {
	tw := &TimeWheel{
		slots:        list.New(),
		stopNotify:   make(chan struct{}),
		updateNotify: make(chan time.Time),
		dispatch:     make(chan TimeSlot, 10),
	}
	go tw.tick()
	return tw
}

func (tw *TimeWheel) Add(target time.Time, value interface{}) {
	if value == nil {
		panic("can't insert nil objects into TimeWheel queue")
	}

	tw.slotsLock.Lock()
	tw.slots.PushBack(TimeSlot{Time: target, Value: value})
	tw.slotsLock.Unlock()

	tw.updateNotify <- target
}

func (tw *TimeWheel) Close() {
	// Idempotent Close is convenient sometimes.
	if tw.stopNotify == nil {
		return
	}

	tw.stopNotify <- struct{}{}
	<-tw.stopNotify

	tw.stopNotify = nil

	close(tw.updateNotify)
	close(tw.dispatch)
}

func (tw *TimeWheel) tick() {
	for {
		now := time.Now()
		// Look for list element closest to now.
		tw.slotsLock.Lock()
		var closestSlot TimeSlot
		var closestEl *list.Element
		for e := tw.slots.Front(); e != nil; e = e.Next() {
			slot := e.Value.(TimeSlot)
			if slot.Time.Sub(now) < closestSlot.Time.Sub(now) || closestSlot.Value == nil {
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

		for {
			select {
			case <-timer.C:
				tw.slotsLock.Lock()
				tw.slots.Remove(closestEl)
				tw.slotsLock.Unlock()
				tw.dispatch <- closestSlot

				// break inside of select exits select, not for loop
				goto breakinnerloop
			case newTarget := <-tw.updateNotify:
				// Avoid unnecessary restarts if new target is not going to affect our
				// current wait time.
				if closestSlot.Time.Sub(now) <= newTarget.Sub(now) {
					continue
				}

				timer.Stop()
				// Recalculate new slot time.
			case <-tw.stopNotify:
				tw.stopNotify <- struct{}{}
				return
			}
		}
	breakinnerloop:
	}
}

func (tw *TimeWheel) Dispatch() <-chan TimeSlot {
	return tw.dispatch
}
