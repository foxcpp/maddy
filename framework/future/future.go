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
	"runtime/debug"
	"sync"

	"github.com/foxcpp/maddy/framework/log"
)

// The Future object implements a container for (value, error) pair that "will
// be populated later" and allows multiple users to wait for it to be set.
//
// It should not be copied after first use.
type Future struct {
	mu  sync.RWMutex
	set bool
	val interface{}
	err error

	notify chan struct{}
}

func New() *Future {
	return &Future{notify: make(chan struct{})}
}

// Set sets the Future (value, error) pair. All currently blocked and future
// Get calls will return it.
func (f *Future) Set(val interface{}, err error) {
	if f == nil {
		panic("nil future used")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.set {
		stack := debug.Stack()
		log.Println("Future.Set called multiple times", stack)
		log.Println("value=", val, "err=", err)
		return
	}

	f.set = true
	f.val = val
	f.err = err

	close(f.notify)
}

func (f *Future) Get() (interface{}, error) {
	if f == nil {
		panic("nil future used")
	}

	return f.GetContext(context.Background())
}

func (f *Future) GetContext(ctx context.Context) (interface{}, error) {
	if f == nil {
		panic("nil future used")
	}

	f.mu.RLock()
	if f.set {
		val := f.val
		err := f.err
		f.mu.RUnlock()
		return val, err
	}

	f.mu.RUnlock()
	select {
	case <-f.notify:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	f.mu.RLock()
	defer f.mu.RUnlock()
	if !f.set {
		panic("future: Notification received, but value is not set")
	}

	return f.val, f.err
}
