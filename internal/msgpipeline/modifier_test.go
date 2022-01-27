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

	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/modify"
	"github.com/foxcpp/maddy/internal/testutils"
)

func TestMsgPipeline_SenderModifier(t *testing.T) {
	target := testutils.Target{}
	modifier := testutils.Modifier{
		InstName: "test_modifier",
		MailFrom: map[string]string{
			"sender@example.com": "sender2@example.com",
		},
	}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			globalModifiers: modify.Group{
				Modifiers: []module.Modifier{modifier},
			},
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

	testutils.CheckTestMessage(t, &target, 0, "sender2@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})

	if modifier.UnclosedStates != 0 {
		t.Fatalf("modifier state objects leak or double-closed, counter: %d", modifier.UnclosedStates)
	}
}

func TestMsgPipeline_SenderModifier_Multiple(t *testing.T) {
	target := testutils.Target{}
	mod1, mod2 := testutils.Modifier{
		InstName: "first_modifier",
		MailFrom: map[string]string{
			"sender@example.com": "sender2@example.com",
		},
	}, testutils.Modifier{
		InstName: "second_modifier",
		MailFrom: map[string]string{
			"sender2@example.com": "sender3@example.com",
		},
	}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			globalModifiers: modify.Group{
				Modifiers: []module.Modifier{mod1, mod2},
			},
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

	testutils.CheckTestMessage(t, &target, 0, "sender3@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})

	if mod1.UnclosedStates != 0 || mod2.UnclosedStates != 0 {
		t.Fatalf("modifier state objects leak or double-closed, counter: %d, %d", mod1.UnclosedStates, mod2.UnclosedStates)
	}
}

func TestMsgPipeline_SenderModifier_PreDispatch(t *testing.T) {
	target := testutils.Target{InstName: "target"}
	mod := testutils.Modifier{
		InstName: "test_modifier",
		MailFrom: map[string]string{
			"sender@example.com": "sender@example.org",
		},
	}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			globalModifiers: modify.Group{
				Modifiers: []module.Modifier{mod},
			},
			perSource: map[string]sourceBlock{
				"example.org": {
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

	testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})

	if len(target.Messages) != 1 {
		t.Fatalf("wrong amount of messages received for target, want %d, got %d", 1, len(target.Messages))
	}
	testutils.CheckTestMessage(t, &target, 0, "sender@example.org", []string{"rcpt1@example.com", "rcpt2@example.com"})

	if mod.UnclosedStates != 0 {
		t.Fatalf("modifier state objects leak or double-closed, counter: %d", mod.UnclosedStates)
	}
}

func TestMsgPipeline_SenderModifier_PostDispatch(t *testing.T) {
	target := testutils.Target{InstName: "target"}
	mod := testutils.Modifier{
		InstName: "test_modifier",
		MailFrom: map[string]string{
			"sender@example.org": "sender@example.com",
		},
	}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{
				"example.org": {
					modifiers: modify.Group{
						Modifiers: []module.Modifier{mod},
					},
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

	testutils.DoTestDelivery(t, &d, "sender@example.org", []string{"rcpt1@example.com", "rcpt2@example.com"})

	if len(target.Messages) != 1 {
		t.Fatalf("wrong amount of messages received for target, want %d, got %d", 1, len(target.Messages))
	}
	testutils.CheckTestMessage(t, &target, 0, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})

	if mod.UnclosedStates != 0 {
		t.Fatalf("modifier state objects leak or double-closed, counter: %d", mod.UnclosedStates)
	}
}

func TestMsgPipeline_SenderModifier_PerRcpt(t *testing.T) {
	// Modifier below will be no-op due to implementation limitations.

	comTarget, orgTarget := testutils.Target{InstName: "com_target"}, testutils.Target{InstName: "org_target"}
	mod := testutils.Modifier{
		InstName: "test_modifier",
		MailFrom: map[string]string{
			"sender@example.com": "sender2@example.com",
		},
	}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{
					"example.com": {
						modifiers: modify.Group{
							Modifiers: []module.Modifier{mod},
						},
						targets: []module.DeliveryTarget{&comTarget},
					},
					"example.org": {
						modifiers: modify.Group{},
						targets:   []module.DeliveryTarget{&orgTarget},
					},
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"rcpt@example.com", "rcpt@example.org"})

	if len(comTarget.Messages) != 1 {
		t.Fatalf("wrong amount of messages received for comTarget, want %d, got %d", 1, len(comTarget.Messages))
	}
	testutils.CheckTestMessage(t, &comTarget, 0, "sender@example.com", []string{"rcpt@example.com"})

	if len(orgTarget.Messages) != 1 {
		t.Fatalf("wrong amount of messages received for orgTarget, want %d, got %d", 1, len(orgTarget.Messages))
	}
	testutils.CheckTestMessage(t, &orgTarget, 0, "sender@example.com", []string{"rcpt@example.org"})

	if mod.UnclosedStates != 0 {
		t.Fatalf("modifier state objects leak or double-closed, counter: %d", mod.UnclosedStates)
	}
}

func TestMsgPipeline_RcptModifier(t *testing.T) {
	target := testutils.Target{}
	mod := testutils.Modifier{
		InstName: "test_modifier",
		RcptTo: map[string][]string{
			"rcpt1@example.com": []string{"rcpt1-alias@example.com"},
			"rcpt2@example.com": []string{"rcpt2-alias@example.com"},
		},
	}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			globalModifiers: modify.Group{
				Modifiers: []module.Modifier{mod},
			},
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

	testutils.CheckTestMessage(t, &target, 0, "sender@example.com", []string{"rcpt1-alias@example.com", "rcpt2-alias@example.com"})

	if mod.UnclosedStates != 0 {
		t.Fatalf("modifier state objects leak or double-closed, counter: %d", mod.UnclosedStates)
	}
}

func TestMsgPipeline_RcptModifier_OriginalRcpt(t *testing.T) {
	target := testutils.Target{}
	mod := testutils.Modifier{
		InstName: "test_modifier",
		RcptTo: map[string][]string{
			"rcpt1@example.com": []string{"rcpt1-alias@example.com"},
			"rcpt2@example.com": []string{"rcpt2-alias@example.com"},
		},
	}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			globalModifiers: modify.Group{
				Modifiers: []module.Modifier{mod},
			},
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

	testutils.CheckTestMessage(t, &target, 0, "sender@example.com", []string{"rcpt1-alias@example.com", "rcpt2-alias@example.com"})
	original1 := target.Messages[0].MsgMeta.OriginalRcpts["rcpt1-alias@example.com"]
	if original1 != "rcpt1@example.com" {
		t.Errorf("wrong OriginalRcpts value for first rcpt, want %s, got %s", "rcpt1@example.com", original1)
	}
	original2 := target.Messages[0].MsgMeta.OriginalRcpts["rcpt2-alias@example.com"]
	if original2 != "rcpt2@example.com" {
		t.Errorf("wrong OriginalRcpts value for first rcpt, want %s, got %s", "rcpt2@example.com", original2)
	}

	if mod.UnclosedStates != 0 {
		t.Fatalf("modifier state objects leak or double-closed, counter: %d", mod.UnclosedStates)
	}
}

func TestMsgPipeline_RcptModifier_OriginalRcpt_Multiple(t *testing.T) {
	target := testutils.Target{}
	mod1, mod2 := testutils.Modifier{
		InstName: "first_modifier",
		RcptTo: map[string][]string{
			"rcpt1@example.com": []string{"rcpt1-alias@example.com"},
			"rcpt2@example.com": []string{"rcpt2-alias@example.com"},
		},
	}, testutils.Modifier{
		InstName: "second_modifier",
		RcptTo: map[string][]string{
			"rcpt1-alias@example.com": []string{"rcpt1-alias2@example.com"},
			"rcpt2@example.com":       []string{"wtf@example.com"},
		},
	}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			globalModifiers: modify.Group{
				Modifiers: []module.Modifier{mod1},
			},
			perSource: map[string]sourceBlock{},
			defaultSource: sourceBlock{
				modifiers: modify.Group{
					Modifiers: []module.Modifier{mod2},
				},
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

	testutils.CheckTestMessage(t, &target, 0, "sender@example.com", []string{"rcpt1-alias2@example.com", "rcpt2-alias@example.com"})
	original1 := target.Messages[0].MsgMeta.OriginalRcpts["rcpt1-alias2@example.com"]
	if original1 != "rcpt1@example.com" {
		t.Errorf("wrong OriginalRcpts value for first rcpt, want %s, got %s", "rcpt1@example.com", original1)
	}
	original2 := target.Messages[0].MsgMeta.OriginalRcpts["rcpt2-alias@example.com"]
	if original2 != "rcpt2@example.com" {
		t.Errorf("wrong OriginalRcpts value for first rcpt, want %s, got %s", "rcpt2@example.com", original2)
	}

	if mod1.UnclosedStates != 0 || mod2.UnclosedStates != 0 {
		t.Fatalf("modifier state objects leak or double-closed, counter: %d, %d", mod1.UnclosedStates, mod2.UnclosedStates)
	}
}

func TestMsgPipeline_RcptModifier_Multiple(t *testing.T) {
	target := testutils.Target{}
	mod1, mod2 := testutils.Modifier{
		InstName: "first_modifier",
		RcptTo: map[string][]string{
			"rcpt1@example.com": []string{"rcpt1-alias@example.com"},
			"rcpt2@example.com": []string{"rcpt2-alias@example.com"},
		},
	}, testutils.Modifier{
		InstName: "second_modifier",
		RcptTo: map[string][]string{
			"rcpt1-alias@example.com": []string{"rcpt1-alias2@example.com"},
			"rcpt2@example.com":       []string{"wtf@example.com"},
		},
	}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			globalModifiers: modify.Group{
				Modifiers: []module.Modifier{mod1, mod2},
			},
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

	testutils.CheckTestMessage(t, &target, 0, "sender@example.com", []string{"rcpt1-alias2@example.com", "rcpt2-alias@example.com"})

	if mod1.UnclosedStates != 0 || mod2.UnclosedStates != 0 {
		t.Fatalf("modifier state objects leak or double-closed, counter: %d, %d", mod1.UnclosedStates, mod2.UnclosedStates)
	}
}

func TestMsgPipeline_RcptModifier_PreDispatch(t *testing.T) {
	target := testutils.Target{}
	mod1, mod2 := testutils.Modifier{
		InstName: "first_modifier",
		RcptTo: map[string][]string{
			"rcpt1@example.com": []string{"rcpt1-alias@example.com"},
			"rcpt2@example.com": []string{"rcpt2-alias@example.com"},
		},
	}, testutils.Modifier{
		InstName: "second_modifier",
		RcptTo: map[string][]string{
			"rcpt1-alias@example.com": []string{"rcpt1-alias2@example.com"},
			"rcpt2@example.com":       []string{"wtf@example.com"},
		},
	}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			globalModifiers: modify.Group{
				Modifiers: []module.Modifier{mod1},
			},
			perSource: map[string]sourceBlock{},
			defaultSource: sourceBlock{
				modifiers: modify.Group{Modifiers: []module.Modifier{mod2}},
				perRcpt: map[string]*rcptBlock{
					"rcpt2-alias@example.com": {
						targets: []module.DeliveryTarget{&target},
					},
					"rcpt1-alias2@example.com": {
						targets: []module.DeliveryTarget{&target},
					},
				},
				defaultRcpt: &rcptBlock{
					rejectErr: errors.New("default rcpt is used"),
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})

	if len(target.Messages) != 1 {
		t.Fatalf("wrong amount of messages received, want %d, got %d", 1, len(target.Messages))
	}

	testutils.CheckTestMessage(t, &target, 0, "sender@example.com", []string{"rcpt1-alias2@example.com", "rcpt2-alias@example.com"})

	if mod1.UnclosedStates != 0 || mod2.UnclosedStates != 0 {
		t.Fatalf("modifier state objects leak or double-closed, counter: %d, %d", mod1.UnclosedStates, mod2.UnclosedStates)
	}
}

func TestMsgPipeline_RcptModifier_PostDispatch(t *testing.T) {
	target := testutils.Target{}
	mod := testutils.Modifier{
		InstName: "test_modifier",
		RcptTo: map[string][]string{
			"rcpt1@example.com": []string{"rcpt1@example.org"},
			"rcpt2@example.com": []string{"rcpt2@example.org"},
		},
	}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{
					"example.com": {
						modifiers: modify.Group{
							Modifiers: []module.Modifier{mod},
						},
						targets: []module.DeliveryTarget{&target},
					},
					"example.org": {
						rejectErr: errors.New("wrong rcpt block is used"),
					},
				},
				defaultRcpt: &rcptBlock{
					rejectErr: errors.New("default rcpt is used"),
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})

	if len(target.Messages) != 1 {
		t.Fatalf("wrong amount of messages received, want %d, got %d", 1, len(target.Messages))
	}

	testutils.CheckTestMessage(t, &target, 0, "sender@example.com", []string{"rcpt1@example.org", "rcpt2@example.org"})

	if mod.UnclosedStates != 0 {
		t.Fatalf("modifier state objects leak or double-closed, counter: %d", mod.UnclosedStates)
	}
}

func TestMsgPipeline_GlobalModifier_Errors(t *testing.T) {
	target := testutils.Target{}
	mod := testutils.Modifier{
		InstName:    "test_modifier",
		InitErr:     errors.New("1"),
		MailFromErr: errors.New("2"),
		RcptToErr:   errors.New("3"),
		BodyErr:     errors.New("4"),
	}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			globalModifiers: modify.Group{Modifiers: []module.Modifier{&mod}},
			perSource:       map[string]sourceBlock{},
			defaultSource: sourceBlock{
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	t.Run("init err", func(t *testing.T) {
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	mod.InitErr = nil

	t.Run("mail from err", func(t *testing.T) {
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	mod.MailFromErr = nil

	t.Run("rcpt to err", func(t *testing.T) {
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	mod.RcptToErr = nil

	t.Run("body err", func(t *testing.T) {
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	mod.BodyErr = nil

	t.Run("no err", func(t *testing.T) {
		testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
	})

	if mod.UnclosedStates != 0 {
		t.Fatalf("modifier state objects leak or double-closed, counter: %d", mod.UnclosedStates)
	}
}

func TestMsgPipeline_SourceModifier_Errors(t *testing.T) {
	target := testutils.Target{}
	mod := testutils.Modifier{
		InstName:    "test_modifier",
		InitErr:     errors.New("1"),
		MailFromErr: errors.New("2"),
		RcptToErr:   errors.New("3"),
		BodyErr:     errors.New("4"),
	}
	// Added to make sure it is freed properly too.
	globalMod := testutils.Modifier{}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource:       map[string]sourceBlock{},
			globalModifiers: modify.Group{Modifiers: []module.Modifier{&globalMod}},
			defaultSource: sourceBlock{
				modifiers: modify.Group{Modifiers: []module.Modifier{&mod}},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	t.Run("init err", func(t *testing.T) {
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	mod.InitErr = nil

	t.Run("mail from err", func(t *testing.T) {
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	mod.MailFromErr = nil

	t.Run("rcpt to err", func(t *testing.T) {
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	mod.RcptToErr = nil

	t.Run("body err", func(t *testing.T) {
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	mod.BodyErr = nil

	t.Run("no err", func(t *testing.T) {
		testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
	})

	if mod.UnclosedStates != 0 || globalMod.UnclosedStates != 0 {
		t.Fatalf("modifier state objects leak or double-closed, counters: %d, %d",
			mod.UnclosedStates, globalMod.UnclosedStates)
	}
}

func TestMsgPipeline_RcptModifier_Errors(t *testing.T) {
	target := testutils.Target{}
	mod := testutils.Modifier{
		InstName:  "test_modifier",
		InitErr:   errors.New("1"),
		RcptToErr: errors.New("3"),
	}
	// Added to make sure it is freed properly too.
	globalMod := testutils.Modifier{}
	sourceMod := testutils.Modifier{}

	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource:       map[string]sourceBlock{},
			globalModifiers: modify.Group{Modifiers: []module.Modifier{&globalMod}},
			defaultSource: sourceBlock{
				modifiers: modify.Group{Modifiers: []module.Modifier{&sourceMod}},
				defaultRcpt: &rcptBlock{
					modifiers: modify.Group{Modifiers: []module.Modifier{&mod}},
					targets:   []module.DeliveryTarget{&target},
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	t.Run("init err", func(t *testing.T) {
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	mod.InitErr = nil

	// MailFromErr test is inapplicable since RewriteSender is not called for per-rcpt
	// modifiers.

	t.Run("rcpt to err", func(t *testing.T) {
		_, err := testutils.DoTestDeliveryErr(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	mod.RcptToErr = nil

	// BodyErr test is inapplicable since RewriteBody is not called for per-rcpt
	// modifiers.

	t.Run("no err", func(t *testing.T) {
		testutils.DoTestDelivery(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
	})

	if mod.UnclosedStates != 0 || globalMod.UnclosedStates != 0 || sourceMod.UnclosedStates != 0 {
		t.Fatalf("modifier state objects leak or double-closed, counters: %d, %d, %d",
			mod.UnclosedStates, globalMod.UnclosedStates, sourceMod.UnclosedStates)
	}
}
