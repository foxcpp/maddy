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

package msgpipeline

import (
	"context"
	"testing"

	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/modify"
	"github.com/foxcpp/maddy/internal/testutils"
)

func streamTestPipeline(t *testing.T, targets ...module.DeliveryTarget) *MsgPipeline {
	return &MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: targets,
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}
}

func TestMsgPipeline_BodyStream(t *testing.T) {
	target := testutils.Target{Streaming: true}
	d := streamTestPipeline(t, &target)

	testutils.DoTestDeliveryStream(t, d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})

	if len(target.Messages) != 1 {
		t.Fatalf("wrong amount of messages received, want %d, got %d", 1, len(target.Messages))
	}
	testutils.CheckTestMessage(t, &target, 0, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
	if !target.Messages[0].Streamed {
		t.Error("message was not streamed")
	}
}

func TestMsgPipeline_CanStreamBody(t *testing.T) {
	canStream := func(t *testing.T, d *MsgPipeline) bool {
		t.Helper()

		delivery, err := d.StartDelivery(context.Background(), &module.MsgMetadata{ID: "test"}, "sender@example.com")
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := delivery.Abort(context.Background()); err != nil {
				t.Error(err)
			}
		}()
		if err := delivery.AddRcpt(context.Background(), "rcpt@example.com", smtp.RcptOptions{}); err != nil {
			t.Fatal(err)
		}
		return delivery.(module.StreamingDelivery).CanStreamBody()
	}

	t.Run("single streaming target", func(t *testing.T) {
		d := streamTestPipeline(t, &testutils.Target{Streaming: true})
		if !canStream(t, d) {
			t.Error("expected streaming to be possible")
		}
	})
	t.Run("target without streaming", func(t *testing.T) {
		d := streamTestPipeline(t, &testutils.Target{})
		if canStream(t, d) {
			t.Error("expected streaming to be impossible")
		}
	})
	t.Run("two targets", func(t *testing.T) {
		d := streamTestPipeline(t,
			&testutils.Target{Streaming: true, InstName: "a"},
			&testutils.Target{Streaming: true, InstName: "b"})
		if canStream(t, d) {
			t.Error("expected streaming to be impossible")
		}
	})
	t.Run("global check", func(t *testing.T) {
		d := streamTestPipeline(t, &testutils.Target{Streaming: true})
		d.globalChecks = []module.Check{&testutils.Check{}}
		if canStream(t, d) {
			t.Error("expected streaming to be impossible")
		}
	})
	t.Run("per-recipient check", func(t *testing.T) {
		d := streamTestPipeline(t, &testutils.Target{Streaming: true})
		d.defaultSource.defaultRcpt.checks = []module.Check{&testutils.Check{}}
		if canStream(t, d) {
			t.Error("expected streaming to be impossible")
		}
	})
	t.Run("source modifier", func(t *testing.T) {
		d := streamTestPipeline(t, &testutils.Target{Streaming: true})
		d.defaultSource.modifiers = modify.Group{Modifiers: []module.Modifier{testutils.Modifier{}}}
		if canStream(t, d) {
			t.Error("expected streaming to be impossible")
		}
	})
	t.Run("dmarc", func(t *testing.T) {
		d := streamTestPipeline(t, &testutils.Target{Streaming: true})
		d.doDMARC = true
		if canStream(t, d) {
			t.Error("expected streaming to be impossible")
		}
	})
}
