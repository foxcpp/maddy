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

package smtp

import (
	"reflect"
	"testing"
	"time"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/framework/module"
)

func init() {
	msgIDField = func() (string, error) {
		return "A", nil
	}

	now = func() time.Time {
		return time.Unix(0, 0)
	}
}

func TestSubmissionPrepare(t *testing.T) {
	test := func(hdrMap, expectedMap map[string][]string) {
		t.Helper()

		hdr := textproto.Header{}
		for k, v := range hdrMap {
			for _, field := range v {
				hdr.Add(k, field)
			}
		}

		endp := testEndpoint(t, "submission", &module.Dummy{}, &module.Dummy{}, nil, nil)
		defer func() {
			// Synchronize the endpoint initialization.
			// Otherwise Close will race with Serve called by setupListeners.
			cl, _ := smtp.Dial("127.0.0.1:" + testPort)
			cl.Close()

			endp.Close()
		}()

		session, err := endp.NewSession(smtp.ConnectionState{}, "")
		if err != nil {
			t.Fatal(err)
		}

		err = session.(*Session).submissionPrepare(&module.MsgMetadata{}, &hdr)
		if expectedMap == nil {
			if err == nil {
				t.Error("Expected an error, got none")
			}
			t.Log(err)
			return
		}
		if expectedMap != nil && err != nil {
			t.Error("Unexpected error:", err)
			return
		}

		resMap := make(map[string][]string)
		for field := hdr.Fields(); field.Next(); {
			resMap[field.Key()] = append(resMap[field.Key()], field.Value())
		}

		if !reflect.DeepEqual(expectedMap, resMap) {
			t.Errorf("wrong header result\nwant %#+v\ngot  %#+v", expectedMap, resMap)
		}
	}

	// No From field.
	test(map[string][]string{}, nil)

	// Malformed From field.
	test(map[string][]string{
		"From": {"<hello@example.org>, \"\""},
	}, nil)
	test(map[string][]string{
		"From": {" adasda"},
	}, nil)

	// Malformed Reply-To.
	test(map[string][]string{
		"From":     {"<hello@example.org>"},
		"Reply-To": {"<hello@example.org>, \"\""},
	}, nil)

	// Malformed CC.
	test(map[string][]string{
		"From":     {"<hello@example.org>"},
		"Reply-To": {"<hello@example.org>"},
		"Cc":       {"<hello@example.org>, \"\""},
	}, nil)

	// Malformed Sender.
	test(map[string][]string{
		"From":     {"<hello@example.org>"},
		"Reply-To": {"<hello@example.org>"},
		"Cc":       {"<hello@example.org>"},
		"Sender":   {"<hello@example.org> asd"},
	}, nil)

	// Multiple From + no Sender.
	test(map[string][]string{
		"From": {"<hello@example.org>, <hello2@example.org>"},
	}, nil)

	// Multiple From + valid Sender.
	test(map[string][]string{
		"From":       {"<hello@example.org>, <hello2@example.org>"},
		"Sender":     {"<hello@example.org>"},
		"Date":       {"Fri, 22 Nov 2019 20:51:31 +0800"},
		"Message-Id": {"<foobar@example.org>"},
	}, map[string][]string{
		"From":       {"<hello@example.org>, <hello2@example.org>"},
		"Sender":     {"<hello@example.org>"},
		"Date":       {"Fri, 22 Nov 2019 20:51:31 +0800"},
		"Message-Id": {"<foobar@example.org>"},
	})

	// Add missing Message-Id.
	test(map[string][]string{
		"From": {"<hello@example.org>"},
		"Date": {"Fri, 22 Nov 2019 20:51:31 +0800"},
	}, map[string][]string{
		"From":       {"<hello@example.org>"},
		"Date":       {"Fri, 22 Nov 2019 20:51:31 +0800"},
		"Message-Id": {"<A@mx.example.com>"},
	})

	// Malformed Date.
	test(map[string][]string{
		"From": {"<hello@example.org>"},
		"Date": {"not a date"},
	}, nil)

	// Add missing Date.
	test(map[string][]string{
		"From":       {"<hello@example.org>"},
		"Message-Id": {"<A@mx.example.org>"},
	}, map[string][]string{
		"From":       {"<hello@example.org>"},
		"Message-Id": {"<A@mx.example.org>"},
		"Date":       {"Thu, 1 Jan 1970 00:00:00 +0000"},
	})
}
