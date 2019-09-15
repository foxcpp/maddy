package future

import (
	"context"
	"testing"
	"time"
)

func TestFuture_SetBeforeGet(t *testing.T) {
	f := New()

	f.Set(1)
	val := f.Get().(int)

	if val != 1 {
		t.Fatal("wrong val received from Get")
	}
}

func TestFuture_Wait(t *testing.T) {
	f := New()

	go func() {
		time.Sleep(500 * time.Millisecond)
		f.Set(1)
	}()

	val := f.Get().(int)
	if val != 1 {
		t.Fatal("wrong val received from Get")
	}

	val = f.Get().(int)
	if val != 1 {
		t.Fatal("wrong val received from Get on second try")
	}
}

func TestFuture_WaitCtx(t *testing.T) {
	f := New()
	ctx, _ := context.WithTimeout(context.Background(), 500*time.Millisecond)
	_, err := f.GetContext(ctx)
	if err != context.DeadlineExceeded {
		t.Fatal("context is not cancelled")
	}
}
