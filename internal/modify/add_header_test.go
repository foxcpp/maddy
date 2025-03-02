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

package modify

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
)

func TestAddHeader(t *testing.T) {
	test := func(headerName string, headerValue string, duplicateHeader bool) {
		t.Helper()

		mod, err := NewAddHeader("modify.add_header", "")
		if err != nil {
			t.Fatal(err)
		}
		m := mod.(*addHeader)
		if err := m.Configure([]string{headerName, headerValue}, config.NewMap(nil, config.Node{})); err != nil {
			t.Fatal(err)
		}

		state, err := m.ModStateForMsg(context.Background(), &module.MsgMetadata{})
		if err != nil {
			t.Fatal(err)
		}

		testHdr := textproto.Header{}
		testHdr.Add("From", "<hello@hello>")
		testHdr.Add("Subject", "heya")
		testHdr.Add("To", "<heya@heya>")
		body := []byte("hello there\r\n")

		// modify.dkim expects RewriteSender to be called to get envelope sender
		//  (see module.Modifier docs)

		err = state.RewriteBody(context.Background(), &testHdr, buffer.MemoryBuffer{Slice: body})
		if err != nil {
			if duplicateHeader && strings.Contains(err.Error(), "already present") {
				return
			}
			t.Fatal(err)
		}
		if duplicateHeader && err == nil {
			t.Fatalf("expected error on duplicate header")
		}

		var fullBody bytes.Buffer
		if err := textproto.WriteHeader(&fullBody, testHdr); err != nil {
			t.Fatal(err)
		}
		if _, err := fullBody.Write(body); err != nil {
			t.Fatal(err)
		}

		if !strings.Contains(fmt.Sprintf("%s", &fullBody), fmt.Sprintf("%s: %s", headerName, headerValue)) {
			t.Fatalf("new header not found in message")
		}
	}

	test("Something", "Somevalue", false)
	test("X-Testing", "Somevalue", false)
	// Test setting a header that is already present
	test("To", "Somevalue", true)
	test("To", "Somevalue", true)
}
