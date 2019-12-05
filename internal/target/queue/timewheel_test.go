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
