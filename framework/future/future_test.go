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
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("context is not cancelled")
	}
}
