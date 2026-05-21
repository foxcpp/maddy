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

package maddy

import (
	"errors"
	"testing"
	"time"

	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/log"
)

// trackingOutput is a log.Output that records whether Close was called.
type trackingOutput struct {
	closed   bool
	closeErr error
}

func (o *trackingOutput) Write(_ time.Time, _ bool, _ string) {}
func (o *trackingOutput) Close() error {
	o.closed = true
	return o.closeErr
}

func newErrLogger() (*log.Logger, *trackingOutput) {
	out := &trackingOutput{}
	return &log.Logger{Out: out}, out
}

// TestCloseContainerLogOutput_NilOut verifies that closeContainerLogOutput
// does not panic when DefaultLogger.Out is nil, which is the configuration
// produced by defaultLogOutput when the user's maddy.conf has no `log`
// directive. Without the nil guard, SIGUSR2 reload panicked in the teardown
// goroutine of moduleReload and the daemon died (foxcpp/maddy#846).
func TestCloseContainerLogOutput_NilOut(t *testing.T) {
	c := &container.C{
		DefaultLogger: &log.Logger{}, // Out is the zero value (nil)
	}
	errLog, errOut := newErrLogger()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("closeContainerLogOutput panicked on nil Out: %v", r)
		}
	}()
	closeContainerLogOutput(c, errLog)

	if errOut.closed {
		t.Fatalf("error logger output should not be closed by the helper")
	}
}

// TestCloseContainerLogOutput_NonNil verifies that a non-nil Out is closed
// exactly once, mirroring the previous unconditional behaviour on the
// happy path.
func TestCloseContainerLogOutput_NonNil(t *testing.T) {
	out := &trackingOutput{}
	c := &container.C{
		DefaultLogger: &log.Logger{Out: out},
	}
	errLog, _ := newErrLogger()

	closeContainerLogOutput(c, errLog)
	if !out.closed {
		t.Fatalf("expected old container log output to be closed")
	}
}

// TestCloseContainerLogOutput_CloseError verifies that an error returned
// from the underlying Close is reported through errLogger and does not
// propagate as a panic.
func TestCloseContainerLogOutput_CloseError(t *testing.T) {
	out := &trackingOutput{closeErr: errors.New("disk full")}
	c := &container.C{
		DefaultLogger: &log.Logger{Out: out},
	}
	errLog, _ := newErrLogger()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("closeContainerLogOutput panicked: %v", r)
		}
	}()
	closeContainerLogOutput(c, errLog)
	if !out.closed {
		t.Fatalf("expected Close to be invoked even when it returns an error")
	}
}

// TestCloseContainerLogOutput_NilContainer verifies the helper tolerates a
// nil container / nil DefaultLogger, so callers do not need to add extra
// nil checks of their own.
func TestCloseContainerLogOutput_NilContainer(t *testing.T) {
	errLog, _ := newErrLogger()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("closeContainerLogOutput panicked on nil container: %v", r)
		}
	}()
	closeContainerLogOutput(nil, errLog)

	c := &container.C{} // DefaultLogger is nil
	closeContainerLogOutput(c, errLog)
}
