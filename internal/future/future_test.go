package future

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestFuture_SetBeforeGet(t *testing.T) {
	f := New()

	f.Set(1, errors.New("1"))
	val, err := f.Get()
	if err.Error() != "1" {
		t.Error("Wrong error:", err)
	}

	if val, _ := val.(int); val != 1 {
		t.Fatal("wrong val received from Get")
	}
}

func TestFuture_Wait(t *testing.T) {
	f := New()

	go func() {
		time.Sleep(500 * time.Millisecond)
		f.Set(1, errors.New("1"))
	}()

	val, err := f.Get()
	if val, _ := val.(int); val != 1 {
		t.Fatal("wrong val received from Get")
	}
	if err.Error() != "1" {
		t.Error("Wrong error:", err)
	}

	val, err = f.Get()
	if val, _ := val.(int); val != 1 {
		t.Fatal("wrong val received from Get on second try")
	}
	if err.Error() != "1" {
		t.Error("Wrong error:", err)
	}

}

func TestFuture_WaitCtx(t *testing.T) {
	f := New()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err := f.GetContext(ctx)
	if err != context.DeadlineExceeded {
		t.Fatal("context is not cancelled")
	}
}
