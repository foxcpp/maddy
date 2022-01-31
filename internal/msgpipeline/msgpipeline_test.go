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
	"context"
	"errors"
	"testing"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/modify"
	"github.com/foxcpp/maddy/internal/testutils"
)

func TestMsgPipeline_AllToTarget(t *testing.T) {
	target := testutils.Target{}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})

	if len(target.Messages) != 1 {
		t.Fatalf("wrong amount of messages received, want %d, got %d", 1, len(target.Messages))
	}

	testutils.CheckTestMessage(t, &target, 0, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
}

func TestMsgPipeline_PerSourceDomainSplit(t *testing.T) {
	orgTarget, comTarget := testutils.Target{InstName: "orgTarget"}, testutils.Target{InstName: "comTarget"}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{
				"example.com": {
					perRcpt: map[string]*rcptBlock{},
					defaultRcpt: &rcptBlock{
						targets: []module.DeliveryTarget{&comTarget},
					},
				},
				"example.org": {
					perRcpt: map[string]*rcptBlock{},
					defaultRcpt: &rcptBlock{
						targets: []module.DeliveryTarget{&orgTarget},
					},
				},
			},
			defaultSource: sourceBlock{rejectErr: errors.New("default src block used")},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
	testutils.DoTestDelivery(t, &d, "sender@example.org", []string{"rcpt1@example.com", "rcpt2@example.com"})

	if len(comTarget.Messages) != 1 {
		t.Fatalf("wrong amount of messages received for comTarget, want %d, got %d", 1, len(comTarget.Messages))
	}
	testutils.CheckTestMessage(t, &comTarget, 0, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})

	if len(orgTarget.Messages) != 1 {
		t.Fatalf("wrong amount of messages received for orgTarget, want %d, got %d", 1, len(orgTarget.Messages))
	}
	testutils.CheckTestMessage(t, &orgTarget, 0, "sender@example.org", []string{"rcpt1@example.com", "rcpt2@example.com"})
}

func TestMsgPipeline_SourceIn(t *testing.T) {
	tblTarget, comTarget := testutils.Target{InstName: "tblTarget"}, testutils.Target{InstName: "comTarget"}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			sourceIn: []sourceIn{
				{
					t:     testutils.Table{},
					block: sourceBlock{rejectErr: errors.New("non-matching block was used")},
				},
				{
					t:     testutils.Table{Err: errors.New("this one will fail")},
					block: sourceBlock{rejectErr: errors.New("failing block was used")},
				},
				{
					t: testutils.Table{
						M: map[string]string{
							"specific@example.com": "",
						},
					},
					block: sourceBlock{
						perRcpt: map[string]*rcptBlock{},
						defaultRcpt: &rcptBlock{
							targets: []module.DeliveryTarget{&tblTarget},
						},
					},
				},
			},
			perSource: map[string]sourceBlock{
				"example.com": {
					perRcpt: map[string]*rcptBlock{},
					defaultRcpt: &rcptBlock{
						targets: []module.DeliveryTarget{&comTarget},
					},
				},
			},
			defaultSource: sourceBlock{rejectErr: errors.New("default src block used")},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"rcpt@example.com"})
	testutils.DoTestDelivery(t, &d, "specific@example.com", []string{"rcpt@example.com"})

	if len(comTarget.Messages) != 1 {
		t.Fatalf("wrong amount of messages received for comTarget, want %d, got %d", 1, len(comTarget.Messages))
	}
	testutils.CheckTestMessage(t, &comTarget, 0, "sender@example.com", []string{"rcpt@example.com"})

	if len(tblTarget.Messages) != 1 {
		t.Fatalf("wrong amount of messages received for orgTarget, want %d, got %d", 1, len(tblTarget.Messages))
	}
	testutils.CheckTestMessage(t, &tblTarget, 0, "specific@example.com", []string{"rcpt@example.com"})
}

func TestMsgPipeline_EmptyMAILFROM(t *testing.T) {
	target := testutils.Target{InstName: "target"}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "", []string{"rcpt1@example.com", "rcpt2@example.com"})

	if len(target.Messages) != 1 {
		t.Fatalf("wrong amount of messages received for target, want %d, got %d", 1, len(target.Messages))
	}
	testutils.CheckTestMessage(t, &target, 0, "", []string{"rcpt1@example.com", "rcpt2@example.com"})
}

func TestMsgPipeline_EmptyMAILFROM_ExplicitDest(t *testing.T) {
	target := testutils.Target{InstName: "target"}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{
				"": {
					perRcpt: map[string]*rcptBlock{},
					defaultRcpt: &rcptBlock{
						targets: []module.DeliveryTarget{&target},
					},
				},
			},
			defaultSource: sourceBlock{rejectErr: errors.New("default src block used")},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "", []string{"rcpt1@example.com", "rcpt2@example.com"})

	if len(target.Messages) != 1 {
		t.Fatalf("wrong amount of messages received for target, want %d, got %d", 1, len(target.Messages))
	}
	testutils.CheckTestMessage(t, &target, 0, "", []string{"rcpt1@example.com", "rcpt2@example.com"})
}

func TestMsgPipeline_PerRcptAddrSplit(t *testing.T) {
	target1, target2 := testutils.Target{InstName: "target1"}, testutils.Target{InstName: "target2"}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{
					"rcpt1@example.com": {
						targets: []module.DeliveryTarget{&target1},
					},
					"rcpt2@example.com": {
						targets: []module.DeliveryTarget{&target2},
					},
				},
				defaultRcpt: &rcptBlock{
					rejectErr: errors.New("defaultRcpt block used"),
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"rcpt1@example.com"})
	testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"rcpt2@example.com"})

	if len(target1.Messages) != 1 {
		t.Errorf("wrong amount of messages received for target1, want %d, got %d", 1, len(target1.Messages))
	}
	testutils.CheckTestMessage(t, &target1, 0, "sender@example.com", []string{"rcpt1@example.com"})

	if len(target2.Messages) != 1 {
		t.Errorf("wrong amount of messages received for target1, want %d, got %d", 1, len(target2.Messages))
	}
	testutils.CheckTestMessage(t, &target2, 0, "sender@example.com", []string{"rcpt2@example.com"})
}

func TestMsgPipeline_PerRcptDomainSplit(t *testing.T) {
	target1, target2 := testutils.Target{InstName: "target1"}, testutils.Target{InstName: "target2"}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{
					"example.com": {
						targets: []module.DeliveryTarget{&target1},
					},
					"example.org": {
						targets: []module.DeliveryTarget{&target2},
					},
				},
				defaultRcpt: &rcptBlock{
					rejectErr: errors.New("defaultRcpt block used"),
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.org"})
	testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"rcpt1@example.org", "rcpt2@example.com"})

	if len(target1.Messages) != 2 {
		t.Errorf("wrong amount of messages received for target1, want %d, got %d", 2, len(target1.Messages))
	}
	testutils.CheckTestMessage(t, &target1, 0, "sender@example.com", []string{"rcpt1@example.com"})
	testutils.CheckTestMessage(t, &target1, 1, "sender@example.com", []string{"rcpt2@example.com"})

	if len(target2.Messages) != 2 {
		t.Errorf("wrong amount of messages received for target2, want %d, got %d", 2, len(target2.Messages))
	}
	testutils.CheckTestMessage(t, &target2, 0, "sender@example.com", []string{"rcpt2@example.org"})
	testutils.CheckTestMessage(t, &target2, 1, "sender@example.com", []string{"rcpt1@example.org"})
}

func TestMsgPipeline_DestInSplit(t *testing.T) {
	target1, target2 := testutils.Target{InstName: "target1"}, testutils.Target{InstName: "target2"}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{},
			defaultSource: sourceBlock{
				rcptIn: []rcptIn{
					{
						t:     testutils.Table{},
						block: &rcptBlock{rejectErr: errors.New("non-matching block was used")},
					},
					{
						t:     testutils.Table{Err: errors.New("nope")},
						block: &rcptBlock{rejectErr: errors.New("failing block was used")},
					},
					{
						t: testutils.Table{
							M: map[string]string{
								"specific@example.com": "",
							},
						},
						block: &rcptBlock{
							targets: []module.DeliveryTarget{&target2},
						},
					},
				},
				perRcpt: map[string]*rcptBlock{
					"example.com": {
						targets: []module.DeliveryTarget{&target1},
					},
				},
				defaultRcpt: &rcptBlock{
					rejectErr: errors.New("defaultRcpt block used"),
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"rcpt1@example.com", "specific@example.com"})

	if len(target1.Messages) != 1 {
		t.Errorf("wrong amount of messages received for target1, want %d, got %d", 1, len(target1.Messages))
	}
	testutils.CheckTestMessage(t, &target1, 0, "sender@example.com", []string{"rcpt1@example.com"})

	if len(target2.Messages) != 1 {
		t.Errorf("wrong amount of messages received for target2, want %d, got %d", 1, len(target2.Messages))
	}
	testutils.CheckTestMessage(t, &target2, 0, "sender@example.com", []string{"specific@example.com"})
}

func TestMsgPipeline_PerSourceAddrAndDomainSplit(t *testing.T) {
	target1, target2 := testutils.Target{InstName: "target1"}, testutils.Target{InstName: "target2"}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{
				"sender1@example.com": {
					perRcpt: map[string]*rcptBlock{},
					defaultRcpt: &rcptBlock{
						targets: []module.DeliveryTarget{&target1},
					},
				},
				"example.com": {
					perRcpt: map[string]*rcptBlock{},
					defaultRcpt: &rcptBlock{
						targets: []module.DeliveryTarget{&target2},
					},
				},
			},
			defaultSource: sourceBlock{rejectErr: errors.New("default src block used")},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "sender1@example.com", []string{"rcpt@example.com"})
	testutils.DoTestDelivery(t, &d, "sender2@example.com", []string{"rcpt@example.com"})

	if len(target1.Messages) != 1 {
		t.Fatalf("wrong amount of messages received for target1, want %d, got %d", 1, len(target1.Messages))
	}
	testutils.CheckTestMessage(t, &target1, 0, "sender1@example.com", []string{"rcpt@example.com"})

	if len(target2.Messages) != 1 {
		t.Fatalf("wrong amount of messages received for target2, want %d, got %d", 1, len(target2.Messages))
	}
	testutils.CheckTestMessage(t, &target2, 0, "sender2@example.com", []string{"rcpt@example.com"})
}

func TestMsgPipeline_PerSourceReject(t *testing.T) {
	target := testutils.Target{}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{
				"sender1@example.com": {
					perRcpt: map[string]*rcptBlock{},
					defaultRcpt: &rcptBlock{
						targets: []module.DeliveryTarget{&target},
					},
				},
				"example.com": {
					perRcpt:   map[string]*rcptBlock{},
					rejectErr: errors.New("go away"),
				},
			},
			defaultSource: sourceBlock{rejectErr: errors.New("go away")},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "sender1@example.com", []string{"rcpt@example.com"})

	_, err := d.Start(context.Background(), &module.MsgMetadata{ID: "testing"}, "sender2@example.com")
	if err == nil {
		t.Error("expected error for delivery.Start, got nil")
	}

	_, err = d.Start(context.Background(), &module.MsgMetadata{ID: "testing"}, "sender2@example.org")
	if err == nil {
		t.Error("expected error for delivery.Start, got nil")
	}
}

func TestMsgPipeline_PerRcptReject(t *testing.T) {
	target := testutils.Target{}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{
					"rcpt1@example.com": {
						targets: []module.DeliveryTarget{&target},
					},
					"example.com": {
						rejectErr: errors.New("go away"),
					},
				},
				defaultRcpt: &rcptBlock{
					rejectErr: errors.New("go away"),
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	delivery, err := d.Start(context.Background(), &module.MsgMetadata{ID: "testing"}, "sender@example.com")
	if err != nil {
		t.Fatalf("unexpected Start err: %v", err)
	}
	defer func() {
		if err := delivery.Abort(context.Background()); err != nil {
			t.Fatalf("unexpected Abort err: %v", err)
		}
	}()

	if err := delivery.AddRcpt(context.Background(), "rcpt2@example.com"); err == nil {
		t.Fatalf("expected error for delivery.AddRcpt(rcpt2@example.com), got nil")
	}
	if err := delivery.AddRcpt(context.Background(), "rcpt1@example.com"); err != nil {
		t.Fatalf("unexpected AddRcpt err for %s: %v", "rcpt1@example.com", err)
	}
	if err := delivery.Body(context.Background(), textproto.Header{}, buffer.MemoryBuffer{Slice: []byte("foobar")}); err != nil {
		t.Fatalf("unexpected Body err: %v", err)
	}
	if err := delivery.Commit(context.Background()); err != nil {
		t.Fatalf("unexpected Commit err: %v", err)
	}
}

func TestMsgPipeline_PostmasterRcpt(t *testing.T) {
	target := testutils.Target{}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{
					"postmaster": {
						targets: []module.DeliveryTarget{&target},
					},
					"example.com": {
						rejectErr: errors.New("go away"),
					},
				},
				defaultRcpt: &rcptBlock{
					rejectErr: errors.New("go away"),
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "disappointed-user@example.com", []string{"postmaster"})
	if len(target.Messages) != 1 {
		t.Fatalf("wrong amount of messages received for target, want %d, got %d", 1, len(target.Messages))
	}
	testutils.CheckTestMessage(t, &target, 0, "disappointed-user@example.com", []string{"postmaster"})
}

func TestMsgPipeline_PostmasterSrc(t *testing.T) {
	target := testutils.Target{}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{
				"postmaster": {
					perRcpt: map[string]*rcptBlock{},
					defaultRcpt: &rcptBlock{
						targets: []module.DeliveryTarget{&target},
					},
				},
				"example.com": {
					rejectErr: errors.New("go away"),
				},
			},
			defaultSource: sourceBlock{
				rejectErr: errors.New("go away"),
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "postmaster", []string{"disappointed-user@example.com"})
	if len(target.Messages) != 1 {
		t.Fatalf("wrong amount of messages received for target, want %d, got %d", 1, len(target.Messages))
	}
	testutils.CheckTestMessage(t, &target, 0, "postmaster", []string{"disappointed-user@example.com"})
}

func TestMsgPipeline_CaseInsensetiveMatch_Src(t *testing.T) {
	target := testutils.Target{}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{
				"postmaster": {
					perRcpt: map[string]*rcptBlock{},
					defaultRcpt: &rcptBlock{
						targets: []module.DeliveryTarget{&target},
					},
				},
				"sender@example.com": {
					perRcpt: map[string]*rcptBlock{},
					defaultRcpt: &rcptBlock{
						targets: []module.DeliveryTarget{&target},
					},
				},
				"example.com": {
					perRcpt: map[string]*rcptBlock{},
					defaultRcpt: &rcptBlock{
						targets: []module.DeliveryTarget{&target},
					},
				},
			},
			defaultSource: sourceBlock{
				rejectErr: errors.New("go away"),
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "POSTMastER", []string{"disappointed-user@example.com"})
	testutils.DoTestDelivery(t, &d, "SenDeR@EXAMPLE.com", []string{"disappointed-user@example.com"})
	testutils.DoTestDelivery(t, &d, "sender@exAMPle.com", []string{"disappointed-user@example.com"})
	if len(target.Messages) != 3 {
		t.Fatalf("wrong amount of messages received for target, want %d, got %d", 3, len(target.Messages))
	}
	testutils.CheckTestMessage(t, &target, 0, "POSTMastER", []string{"disappointed-user@example.com"})
	testutils.CheckTestMessage(t, &target, 1, "SenDeR@EXAMPLE.com", []string{"disappointed-user@example.com"})
	testutils.CheckTestMessage(t, &target, 2, "sender@exAMPle.com", []string{"disappointed-user@example.com"})
}

func TestMsgPipeline_CaseInsensetiveMatch_Rcpt(t *testing.T) {
	target := testutils.Target{}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{
					"postmaster": {
						targets: []module.DeliveryTarget{&target},
					},
					"sender@example.com": {
						targets: []module.DeliveryTarget{&target},
					},
					"example.com": {
						targets: []module.DeliveryTarget{&target},
					},
				},
				defaultRcpt: &rcptBlock{
					rejectErr: errors.New("wtf"),
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"POSTMastER"})
	testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"SenDeR@EXAMPLE.com"})
	testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"sender@exAMPle.com"})
	if len(target.Messages) != 3 {
		t.Fatalf("wrong amount of messages received for target, want %d, got %d", 3, len(target.Messages))
	}
	testutils.CheckTestMessage(t, &target, 0, "sender@example.com", []string{"POSTMastER"})
	testutils.CheckTestMessage(t, &target, 1, "sender@example.com", []string{"SenDeR@EXAMPLE.com"})
	testutils.CheckTestMessage(t, &target, 2, "sender@example.com", []string{"sender@exAMPle.com"})
}

func TestMsgPipeline_UnicodeNFC_Rcpt(t *testing.T) {
	target := testutils.Target{}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{
					"rcpt@é.example.com": {
						targets: []module.DeliveryTarget{&target},
					},
					"é.example.com": {
						targets: []module.DeliveryTarget{&target},
					},
				},
				defaultRcpt: &rcptBlock{
					rejectErr: errors.New("wtf"),
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"rcpt@E\u0301.EXAMPLE.com"})
	testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"f@E\u0301.exAMPle.com"})
	if len(target.Messages) != 2 {
		t.Fatalf("wrong amount of messages received for target, want %d, got %d", 2, len(target.Messages))
	}
	testutils.CheckTestMessage(t, &target, 0, "sender@example.com", []string{"rcpt@E\u0301.EXAMPLE.com"})
	testutils.CheckTestMessage(t, &target, 1, "sender@example.com", []string{"f@E\u0301.exAMPle.com"})
}

func TestMsgPipeline_MalformedSource(t *testing.T) {
	target := testutils.Target{}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{
					"postmaster": {
						targets: []module.DeliveryTarget{&target},
					},
					"sender@example.com": {
						targets: []module.DeliveryTarget{&target},
					},
					"example.com": {
						targets: []module.DeliveryTarget{&target},
					},
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	// Simple checks for violations that can make msgpipeline misbehave.
	for _, addr := range []string{"not_postmaster_but_no_at_sign", "@no_mailbox", "no_domain@"} {
		_, err := d.Start(context.Background(), &module.MsgMetadata{ID: "testing"}, addr)
		if err == nil {
			t.Errorf("%s is accepted as valid address", addr)
		}
	}
}

func TestMsgPipeline_TwoRcptToOneTarget(t *testing.T) {
	target := testutils.Target{}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{
					"example.com": {
						targets: []module.DeliveryTarget{&target},
					},
					"example.org": {
						targets: []module.DeliveryTarget{&target},
					},
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"recipient@example.com", "recipient@example.org"})

	if len(target.Messages) != 1 {
		t.Fatalf("wrong amount of messages received for target, want %d, got %d", 1, len(target.Messages))
	}
	testutils.CheckTestMessage(t, &target, 0, "sender@example.com", []string{"recipient@example.com", "recipient@example.org"})
}

func TestMsgPipeline_multi_alias(t *testing.T) {
	target1, target2 := testutils.Target{InstName: "target1"}, testutils.Target{InstName: "target2"}
	mod := testutils.Modifier{
		RcptTo: map[string][]string{
			"recipient@example.com": []string{
				"recipient-1@example.org",
				"recipient-2@example.net",
			},
		},
	}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{},
			defaultSource: sourceBlock{
				modifiers: modify.Group{
					Modifiers: []module.Modifier{mod},
				},
				perRcpt: map[string]*rcptBlock{
					"example.org": {
						targets: []module.DeliveryTarget{&target1},
					},
					"example.net": {
						targets: []module.DeliveryTarget{&target2},
					},
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"recipient@example.com"})

	if len(target1.Messages) != 1 {
		t.Errorf("wrong amount of messages received for target1, want %d, got %d", 1, len(target1.Messages))
	}
	testutils.CheckTestMessage(t, &target1, 0, "sender@example.com", []string{"recipient-1@example.org"})

	if len(target2.Messages) != 1 {
		t.Errorf("wrong amount of messages received for target1, want %d, got %d", 1, len(target2.Messages))
	}
	testutils.CheckTestMessage(t, &target2, 0, "sender@example.com", []string{"recipient-2@example.net"})
}
