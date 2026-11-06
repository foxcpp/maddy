/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2025 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

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

package ignore_error

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/testutils"
)

func wrap(t *testing.T, inner *testutils.Target) *Target {
	return &Target{
		instName: "test",
		log:      testutils.Logger(t, modName),
		wrapped:  inner,
	}
}

func TestPassthroughOnSuccess(t *testing.T) {
	inner := &testutils.Target{}
	tgt := wrap(t, inner)

	testutils.DoTestDelivery(t, tgt, "from@example.org", []string{"to@example.org"})

	testutils.CheckTestMessage(t, inner, 0, "from@example.org", []string{"to@example.org"})
}

func TestIgnoredErrors(t *testing.T) {
	boom := errors.New("boom")

	cases := map[string]*testutils.Target{
		"StartDelivery": {StartErr: boom},
		"AddRcpt":       {RcptErr: map[string]error{"to@example.org": boom}},
		"Body":          {BodyErr: boom},
		"Commit":        {CommitErr: boom},
	}

	for name, inner := range cases {
		t.Run(name, func(t *testing.T) {
			tgt := wrap(t, inner)

			if _, err := testutils.DoTestDeliveryErr(t, tgt, "from@example.org", []string{"to@example.org"}); err != nil {
				t.Errorf("wrapped %s error was not ignored: %v", name, err)
			}
		})
	}
}

func TestBodyStream(t *testing.T) {
	inner := &testutils.Target{Streaming: true}
	tgt := wrap(t, inner)

	testutils.DoTestDeliveryStream(t, tgt, "from@example.org", []string{"to@example.org"})

	testutils.CheckTestMessage(t, inner, 0, "from@example.org", []string{"to@example.org"})
	if !inner.Messages[0].Streamed {
		t.Error("message body was not streamed to the wrapped target")
	}
}

func TestBodyStreamNotSupported(t *testing.T) {
	tgt := wrap(t, &testutils.Target{})

	d, err := tgt.StartDelivery(context.Background(), &module.MsgMetadata{}, "from@example.org")
	if err != nil {
		t.Fatal(err)
	}
	if d.(module.StreamingDelivery).CanStreamBody() {
		t.Error("CanStreamBody should be false if the wrapped delivery can't stream")
	}
}

func startStream(t *testing.T, tgt *Target) module.StreamingDelivery {
	t.Helper()

	d, err := tgt.StartDelivery(context.Background(), &module.MsgMetadata{}, "from@example.org")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.AddRcpt(context.Background(), "to@example.org", smtp.RcptOptions{}); err != nil {
		t.Fatal(err)
	}
	streamDelivery := d.(module.StreamingDelivery)
	if !streamDelivery.CanStreamBody() {
		t.Fatal("CanStreamBody is false")
	}
	return streamDelivery
}

func TestBodyStreamIgnoredError(t *testing.T) {
	boom := errors.New("boom")

	for name, inner := range map[string]*testutils.Target{
		"StartDelivery": {Streaming: true, StartErr: boom},
		"BodyStream":    {Streaming: true, BodyErr: boom},
	} {
		t.Run(name, func(t *testing.T) {
			d := startStream(t, wrap(t, inner))

			body := strings.NewReader("foobar\r\n")
			if err := d.BodyStream(context.Background(), textproto.Header{}, body); err != nil {
				t.Errorf("wrapped error was not ignored: %v", err)
			}
			if body.Len() != 0 {
				t.Errorf("body was not read to the end, %d bytes left", body.Len())
			}
		})
	}
}

func TestBodyStreamReadError(t *testing.T) {
	readErr := errors.New("client went away")

	for name, inner := range map[string]*testutils.Target{
		"Passthrough": {Streaming: true},
		"BodyStream":  {Streaming: true, BodyErr: errors.New("boom")},
	} {
		t.Run(name, func(t *testing.T) {
			d := startStream(t, wrap(t, inner))

			body := io.MultiReader(strings.NewReader("foo"), iotest.ErrReader(readErr))
			if err := d.BodyStream(context.Background(), textproto.Header{}, body); !errors.Is(err, readErr) {
				t.Errorf("expected the read error, got %v", err)
			}
		})
	}
}

func TestConfigureRequiresTarget(t *testing.T) {
	tgt := &Target{instName: "test", log: testutils.Logger(t, modName)}

	if err := tgt.Configure(nil, config.NewMap(nil, config.Node{})); err == nil {
		t.Error("expected an error when no wrapped target is given")
	}
}
