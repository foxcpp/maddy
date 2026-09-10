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

package milter

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/emersion/go-milter"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
)

func TestAcceptValidEndpoints(t *testing.T) {
	for _, endpoint := range []string{
		"tcp://0.0.0.0:10025",
		"tcp://[::]:10025",
		"tcp:127.0.0.1:10025",
		"unix://path",
		"unix:path",
		"unix:/path",
		"unix:///path",
		"unix://also/path",
		"unix:///also/path",
	} {
		c := &Check{milterUrl: endpoint}

		err := c.Configure(nil, &config.Map{})
		if err != nil {
			t.Errorf("Unexpected failure for %s: %v", endpoint, err)
			return
		}
	}
}

func TestRejectInvalidEndpoints(t *testing.T) {
	for _, endpoint := range []string{
		"tls://0.0.0.0:10025",
		"tls:0.0.0.0:10025",
	} {
		c := &Check{milterUrl: endpoint}
		err := c.Configure(nil, &config.Map{})
		if err == nil {
			t.Errorf("Accepted invalid endpoint: %s", endpoint)
			return
		}
	}
}

// unreachableAddr returns a loopback TCP address that is guaranteed to
// refuse connections: it binds a listener to get a free port, then closes
// it immediately, so dialing it fails fast with ECONNREFUSED instead of
// waiting out a dial timeout.
func unreachableAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate a port to make unreachable: %v", err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatalf("failed to close listener: %v", err)
	}
	return addr
}

func newUnreachableCheck(t *testing.T, failOpen bool) *Check {
	t.Helper()
	return &Check{
		failOpen: failOpen,
		log:      &log.Logger{Out: log.NopOutput{}},
		cl: milter.NewClientWithOptions("tcp", unreachableAddr(t), milter.ClientOptions{
			Dialer:       &net.Dialer{Timeout: 2 * time.Second},
			ReadTimeout:  2 * time.Second,
			WriteTimeout: 2 * time.Second,
		}),
	}
}

// TestCheckStateForMsg_DialFailure_FailOpenFalse locks in the existing,
// correct behavior: without fail_open, an unreachable milter must still
// hard-reject at state-creation time.
func TestCheckStateForMsg_DialFailure_FailOpenFalse(t *testing.T) {
	c := newUnreachableCheck(t, false)
	_, err := c.CheckStateForMsg(context.Background(), &module.MsgMetadata{ID: "test"})
	if err == nil {
		t.Fatal("expected an error when the milter is unreachable and fail_open is false, got nil")
	}
}

// TestCheckStateForMsg_DialFailure_FailOpenTrue reproduces the fail_open
// gap: a dial failure while establishing the milter session used to bypass
// fail_open entirely (CheckStateForMsg returned the raw dial error
// unconditionally), hard-rejecting the message despite fail_open being set.
// It must instead behave like ioError() does for a later I/O failure: let
// the message through unchecked.
func TestCheckStateForMsg_DialFailure_FailOpenTrue(t *testing.T) {
	c := newUnreachableCheck(t, true)

	st, err := c.CheckStateForMsg(context.Background(), &module.MsgMetadata{ID: "test"})
	if err != nil {
		t.Fatalf("expected fail_open to let the message through despite the unreachable milter, got error: %v", err)
	}
	if st == nil {
		t.Fatal("expected a non-nil check state with fail_open, got nil")
	}
	defer func() {
		if err := st.Close(); err != nil {
			t.Errorf("Close: expected nil error on a sessionless (dial-failed) state, got: %v", err)
		}
	}()

	if res := st.CheckConnection(context.Background()); res.Reject {
		t.Errorf("CheckConnection: expected no rejection with fail_open after a dial failure, got Reject=true, reason=%v", res.Reason)
	}
	if res := st.CheckSender(context.Background(), "sender@example.org"); res.Reject {
		t.Errorf("CheckSender: expected no rejection with fail_open after a dial failure, got Reject=true, reason=%v", res.Reason)
	}
	if res := st.CheckRcpt(context.Background(), "rcpt@example.org"); res.Reject {
		t.Errorf("CheckRcpt: expected no rejection with fail_open after a dial failure, got Reject=true, reason=%v", res.Reason)
	}
}
