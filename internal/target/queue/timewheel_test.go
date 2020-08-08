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
	"testing"
	"time"
)

func TestTimeWheelAdd(t *testing.T) {
	t.Parallel()

	called := make(chan TimeSlot)

	w := NewTimeWheel(func(slot TimeSlot) {
		called <- slot
	})
	defer w.Close()

	w.Add(time.Now().Add(1*time.Second), 1)

	slot := <-called
	if val, _ := slot.Value.(int); val != 1 {
		t.Errorf("Wrong slot value: %v", slot.Value)
	}
}

func TestTimeWheelAdd_Ordering(t *testing.T) {
	t.Parallel()

	called := make(chan TimeSlot)

	w := NewTimeWheel(func(slot TimeSlot) {
		called <- slot
	})
	defer w.Close()

	w.Add(time.Now().Add(1*time.Second), 1)
	w.Add(time.Now().Add(1250*time.Millisecond), 2)

	slot := <-called
	if val, _ := slot.Value.(int); val != 1 {
		t.Errorf("Wrong first slot value: %v", slot.Value)
	}
	slot = <-called
	if val, _ := slot.Value.(int); val != 2 {
		t.Errorf("Wrong second slot value: %v", slot.Value)
	}
}

func TestTimeWheelAdd_Restart(t *testing.T) {
	t.Parallel()

	called := make(chan TimeSlot)

	w := NewTimeWheel(func(slot TimeSlot) {
		called <- slot
	})
	defer w.Close()

	w.Add(time.Now().Add(1*time.Second), 1)
	w.Add(time.Now().Add(500*time.Millisecond), 2)

	slot := <-called
	if val, _ := slot.Value.(int); val != 2 {
		t.Errorf("Wrong first slot value: %v", slot.Value)
	}
	slot = <-called
	if val, _ := slot.Value.(int); val != 1 {
		t.Errorf("Wrong second slot value: %v", slot.Value)
	}
}

func TestTimeWheelAdd_MissingGotoBug(t *testing.T) {
	t.Parallel()

	called := make(chan TimeSlot)

	w := NewTimeWheel(func(slot TimeSlot) {
		called <- slot
	})
	defer w.Close()

	w.Add(time.Now().Add(90000*time.Hour), 1)      // practically newer
	w.Add(time.Now().Add(500*time.Millisecond), 2) // should correctly restart

	slot := <-called
	if val, _ := slot.Value.(int); val != 2 {
		t.Errorf("Wrong first slot value: %v", slot.Value)
	}
}

func TestTimeWheelAdd_EmptyUpdWait(t *testing.T) {
	t.Parallel()

	called := make(chan TimeSlot)

	w := NewTimeWheel(func(slot TimeSlot) {
		called <- slot
	})
	defer w.Close()

	time.Sleep(500 * time.Millisecond)

	w.Add(time.Now().Add(1*time.Second), 1)

	slot := <-called
	if val, _ := slot.Value.(int); val != 1 {
		t.Errorf("Wrong slot value: %v", slot.Value)
	}
}
