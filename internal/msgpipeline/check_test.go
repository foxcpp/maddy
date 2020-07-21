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

package msgpipeline

import (
	"errors"
	"testing"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-msgauth/authres"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/testutils"
)

func TestMsgPipeline_Checks(t *testing.T) {
	target := testutils.Target{}
	check1, check2 := testutils.Check{}, testutils.Check{}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			globalChecks: []module.Check{&check1, &check2},
			perSource:    map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "whatever@whatever", []string{"whatever@whatever"})

	if len(target.Messages) != 1 {
		t.Fatalf("wrong amount of messages received, want %d, got %d", 1, len(target.Messages))
	}
	if target.Messages[0].MsgMeta.Quarantine {
		t.Fatalf("message is quarantined when it shouldn't")
	}

	if check1.UnclosedStates != 0 || check2.UnclosedStates != 0 {
		t.Fatalf("checks state objects leak or double-closed, alive counters: %v, %v", check1.UnclosedStates, check2.UnclosedStates)
	}
}

func TestMsgPipeline_AuthResults(t *testing.T) {
	target := testutils.Target{}
	check1, check2 := testutils.Check{
		BodyRes: module.CheckResult{
			AuthResult: []authres.Result{
				&authres.SPFResult{
					Value: authres.ResultFail,
					From:  "FROM",
					Helo:  "HELO",
				},
			},
		},
	}, testutils.Check{
		BodyRes: module.CheckResult{
			AuthResult: []authres.Result{
				&authres.SPFResult{
					Value: authres.ResultFail,
					From:  "FROM2",
					Helo:  "HELO2",
				},
			},
		},
	}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			globalChecks: []module.Check{&check1, &check2},
			perSource:    map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
		},
		Hostname: "TEST-HOST",
		Log:      testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "whatever@whatever", []string{"whatever@whatever"})

	if len(target.Messages) != 1 {
		t.Fatalf("wrong amount of messages received, want %d, got %d", 1, len(target.Messages))
	}

	authRes := target.Messages[0].Header.Get("Authentication-Results")
	id, parsed, err := authres.Parse(authRes)
	if err != nil {
		t.Fatalf("failed to parse results")
	}
	if id != "TEST-HOST" {
		t.Fatalf("wrong authres identifier")
	}
	if len(parsed) != 2 {
		t.Fatalf("wrong amount of parts, want %d, got %d", 2, len(parsed))
	}

	var seen1, seen2 bool
	for _, parts := range parsed {
		spfPart, ok := parts.(*authres.SPFResult)
		if !ok {
			t.Fatalf("Not SPFResult")
		}

		if spfPart.From == "FROM" {
			seen1 = true
		}
		if spfPart.From == "FROM2" {
			seen2 = true
		}
	}

	if !seen1 {
		t.Fatalf("First authRes is missing")
	}
	if !seen2 {
		t.Fatalf("Second authRes is missing")
	}

	if check1.UnclosedStates != 0 || check2.UnclosedStates != 0 {
		t.Fatalf("checks state objects leak or double-closed, alive counters: %v, %v", check1.UnclosedStates, check2.UnclosedStates)
	}
}

func TestMsgPipeline_Headers(t *testing.T) {
	hdr1 := textproto.Header{}
	hdr1.Add("HDR1", "1")
	hdr2 := textproto.Header{}
	hdr2.Add("HDR2", "2")

	target := testutils.Target{}
	check1, check2 := testutils.Check{
		BodyRes: module.CheckResult{
			Header: hdr1,
		},
	}, testutils.Check{
		BodyRes: module.CheckResult{
			Header: hdr2,
		},
	}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			globalChecks: []module.Check{&check1, &check2},
			perSource:    map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
		},
		Hostname: "TEST-HOST",
		Log:      testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "whatever@whatever", []string{"whatever@whatever"})

	if len(target.Messages) != 1 {
		t.Fatalf("wrong amount of messages received, want %d, got %d", 1, len(target.Messages))
	}

	if target.Messages[0].Header.Get("HDR1") != "1" {
		t.Fatalf("wrong HDR1 value, want %s, got %s", "1", target.Messages[0].Header.Get("HDR1"))
	}
	if target.Messages[0].Header.Get("HDR2") != "2" {
		t.Fatalf("wrong HDR2 value, want %s, got %s", "1", target.Messages[0].Header.Get("HDR2"))
	}

	if check1.UnclosedStates != 0 || check2.UnclosedStates != 0 {
		t.Fatalf("checks state objects leak or double-closed, alive counters: %v, %v", check1.UnclosedStates, check2.UnclosedStates)
	}
}

func TestMsgPipeline_Globalcheck_Errors(t *testing.T) {
	target := testutils.Target{}
	check_ := testutils.Check{
		InitErr:   errors.New("1"),
		ConnRes:   module.CheckResult{Reject: true, Reason: errors.New("2")},
		SenderRes: module.CheckResult{Reject: true, Reason: errors.New("3")},
		RcptRes:   module.CheckResult{Reject: true, Reason: errors.New("4")},
		BodyRes:   module.CheckResult{Reject: true, Reason: errors.New("5")},
	}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			globalChecks: []module.Check{&check_},
			perSource:    map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
		},
		Hostname: "TEST-HOST",
		Log:      testutils.Logger(t, "msgpipeline"),
	}

	t.Run("init err", func(t *testing.T) {
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	check_.InitErr = nil

	t.Run("conn err", func(t *testing.T) {
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	check_.ConnRes.Reject = false

	t.Run("mail from err", func(t *testing.T) {
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	check_.SenderRes.Reject = false

	t.Run("rcpt to err", func(t *testing.T) {
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	check_.RcptRes.Reject = false

	t.Run("body err", func(t *testing.T) {
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	check_.BodyRes.Reject = false

	t.Run("no err", func(t *testing.T) {
		testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
	})

	if check_.UnclosedStates != 0 {
		t.Fatalf("check state objects leak or double-closed, counters: %d", check_.UnclosedStates)
	}
}

func TestMsgPipeline_SourceCheck_Errors(t *testing.T) {
	target := testutils.Target{}
	check_ := testutils.Check{
		InitErr:   errors.New("1"),
		ConnRes:   module.CheckResult{Reject: true, Reason: errors.New("2")},
		SenderRes: module.CheckResult{Reject: true, Reason: errors.New("3")},
		RcptRes:   module.CheckResult{Reject: true, Reason: errors.New("4")},
		BodyRes:   module.CheckResult{Reject: true, Reason: errors.New("5")},
	}
	globalCheck := testutils.Check{}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			globalChecks: []module.Check{&globalCheck},
			perSource:    map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				checks:  []module.Check{&check_},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
		},
		Hostname: "TEST-HOST",
		Log:      testutils.Logger(t, "msgpipeline"),
	}

	t.Run("init err", func(t *testing.T) {
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	check_.InitErr = nil

	t.Run("conn err", func(t *testing.T) {
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	check_.ConnRes.Reject = false

	t.Run("mail from err", func(t *testing.T) {
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	check_.SenderRes.Reject = false

	t.Run("rcpt to err", func(t *testing.T) {
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	check_.RcptRes.Reject = false

	t.Run("body err", func(t *testing.T) {
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	check_.BodyRes.Reject = false

	t.Run("no err", func(t *testing.T) {
		testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
	})

	if check_.UnclosedStates != 0 || globalCheck.UnclosedStates != 0 {
		t.Fatalf("check state objects leak or double-closed, counters: %d, %d",
			check_.UnclosedStates, globalCheck.UnclosedStates)
	}
}

func TestMsgPipeline_RcptCheck_Errors(t *testing.T) {
	target := testutils.Target{}
	check_ := testutils.Check{
		InitErr:   errors.New("1"),
		ConnRes:   module.CheckResult{Reject: true, Reason: errors.New("2")},
		SenderRes: module.CheckResult{Reject: true, Reason: errors.New("3")},
		RcptRes:   module.CheckResult{Reject: true, Reason: errors.New("4")},
		BodyRes:   module.CheckResult{Reject: true, Reason: errors.New("5")},

		InstName: "err_check",
	}
	// Added to check whether it leaks.
	globalCheck := testutils.Check{InstName: "global_check"}
	sourceCheck := testutils.Check{InstName: "source_check"}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			globalChecks: []module.Check{&globalCheck},
			perSource:    map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				checks:  []module.Check{&check_},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
		},
		Hostname: "TEST-HOST",
		Log:      testutils.Logger(t, "msgpipeline"),
	}

	t.Run("init err", func(t *testing.T) {
		d.Log = testutils.Logger(t, "msgpipeline")
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}

		t.Log("!!!", check_.UnclosedStates)
	})

	check_.InitErr = nil

	t.Run("conn err", func(t *testing.T) {
		d.Log = testutils.Logger(t, "msgpipeline")
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}

		t.Log("!!!", check_.UnclosedStates)
	})

	check_.ConnRes.Reject = false

	t.Run("mail from err", func(t *testing.T) {
		d.Log = testutils.Logger(t, "msgpipeline")
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}

		t.Log("!!!", check_.UnclosedStates)
	})

	check_.SenderRes.Reject = false

	t.Run("rcpt to err", func(t *testing.T) {
		d.Log = testutils.Logger(t, "msgpipeline")
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	check_.RcptRes.Reject = false

	t.Run("body err", func(t *testing.T) {
		d.Log = testutils.Logger(t, "msgpipeline")
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	check_.BodyRes.Reject = false

	t.Run("no err", func(t *testing.T) {
		d.Log = testutils.Logger(t, "msgpipeline")
		testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
	})

	if check_.UnclosedStates != 0 || sourceCheck.UnclosedStates != 0 || globalCheck.UnclosedStates != 0 {
		t.Fatalf("check state objects leak or double-closed, counters: %d, %d, %d",
			check_.UnclosedStates, sourceCheck.UnclosedStates, globalCheck.UnclosedStates)
	}
}
