package maddy

import (
	"errors"
	"testing"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/maddy/buffer"
	"github.com/emersion/maddy/module"
)

func TestDispatcher_AllToTarget(t *testing.T) {
	target := testTarget{}
	d := Dispatcher{
		dispatcherCfg: dispatcherCfg{
			perSource: map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
		},
		Log: testLogger(t, "dispatcher"),
	}

	doTestDelivery(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})

	if len(target.messages) != 1 {
		t.Fatalf("wrong amount of messages received, want %d, got %d", 1, len(target.messages))
	}

	checkTestMessage(t, &target, 0, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
}

func TestDispatcher_PerSourceDomainSplit(t *testing.T) {
	orgTarget, comTarget := testTarget{instName: "orgTarget"}, testTarget{instName: "comTarget"}
	d := Dispatcher{
		dispatcherCfg: dispatcherCfg{
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
		Log: testLogger(t, "dispatcher"),
	}

	doTestDelivery(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})
	doTestDelivery(t, &d, "sender@example.org", []string{"rcpt1@example.com", "rcpt2@example.com"})

	if len(comTarget.messages) != 1 {
		t.Fatalf("wrong amount of messages received for comTarget, want %d, got %d", 1, len(comTarget.messages))
	}
	checkTestMessage(t, &comTarget, 0, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.com"})

	if len(orgTarget.messages) != 1 {
		t.Fatalf("wrong amount of messages received for orgTarget, want %d, got %d", 1, len(orgTarget.messages))
	}
	checkTestMessage(t, &orgTarget, 0, "sender@example.org", []string{"rcpt1@example.com", "rcpt2@example.com"})
}

func TestDispatcher_PerRcptAddrSplit(t *testing.T) {
	target1, target2 := testTarget{instName: "target1"}, testTarget{instName: "target2"}
	d := Dispatcher{
		dispatcherCfg: dispatcherCfg{
			perSource: map[string]sourceBlock{
				"sender1@example.com": {
					perRcpt: map[string]*rcptBlock{},
					defaultRcpt: &rcptBlock{
						targets: []module.DeliveryTarget{&target1},
					},
				},
				"sender2@example.com": {
					perRcpt: map[string]*rcptBlock{},
					defaultRcpt: &rcptBlock{
						targets: []module.DeliveryTarget{&target2},
					},
				},
			},
			defaultSource: sourceBlock{rejectErr: errors.New("default src block used")},
		},
		Log: testLogger(t, "dispatcher"),
	}

	doTestDelivery(t, &d, "sender1@example.com", []string{"rcpt@example.com"})
	doTestDelivery(t, &d, "sender2@example.com", []string{"rcpt@example.com"})

	if len(target1.messages) != 1 {
		t.Errorf("wrong amount of messages received for target1, want %d, got %d", 1, len(target1.messages))
	}
	checkTestMessage(t, &target1, 0, "sender1@example.com", []string{"rcpt@example.com"})

	if len(target2.messages) != 1 {
		t.Errorf("wrong amount of messages received for target1, want %d, got %d", 1, len(target2.messages))
	}
	checkTestMessage(t, &target2, 0, "sender2@example.com", []string{"rcpt@example.com"})
}

func TestDispatcher_PerRcptDomainSplit(t *testing.T) {
	target1, target2 := testTarget{instName: "target1"}, testTarget{instName: "target2"}
	d := Dispatcher{
		dispatcherCfg: dispatcherCfg{
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
		Log: testLogger(t, "dispatcher"),
	}

	doTestDelivery(t, &d, "sender@example.com", []string{"rcpt1@example.com", "rcpt2@example.org"})
	doTestDelivery(t, &d, "sender@example.com", []string{"rcpt1@example.org", "rcpt2@example.com"})

	if len(target1.messages) != 2 {
		t.Errorf("wrong amount of messages received for target1, want %d, got %d", 2, len(target1.messages))
	}
	checkTestMessage(t, &target1, 0, "sender@example.com", []string{"rcpt1@example.com"})
	checkTestMessage(t, &target1, 1, "sender@example.com", []string{"rcpt2@example.com"})

	if len(target2.messages) != 2 {
		t.Errorf("wrong amount of messages received for target2, want %d, got %d", 2, len(target2.messages))
	}
	checkTestMessage(t, &target2, 0, "sender@example.com", []string{"rcpt2@example.org"})
	checkTestMessage(t, &target2, 1, "sender@example.com", []string{"rcpt1@example.org"})
}

func TestDispatcher_PerSourceAddrAndDomainSplit(t *testing.T) {
	target1, target2 := testTarget{instName: "target1"}, testTarget{instName: "target2"}
	d := Dispatcher{
		dispatcherCfg: dispatcherCfg{
			perSource: map[string]sourceBlock{
				"sender1@example.com": {
					perRcpt: map[string]*rcptBlock{},
					defaultRcpt: &rcptBlock{
						targets: []module.DeliveryTarget{&target1},
					},
				},
				"example.com": {perRcpt: map[string]*rcptBlock{},
					defaultRcpt: &rcptBlock{
						targets: []module.DeliveryTarget{&target2},
					},
				},
			},
			defaultSource: sourceBlock{rejectErr: errors.New("default src block used")},
		},
		Log: testLogger(t, "dispatcher"),
	}

	doTestDelivery(t, &d, "sender1@example.com", []string{"rcpt@example.com"})
	doTestDelivery(t, &d, "sender2@example.com", []string{"rcpt@example.com"})

	if len(target1.messages) != 1 {
		t.Fatalf("wrong amount of messages received for target1, want %d, got %d", 1, len(target1.messages))
	}
	checkTestMessage(t, &target1, 0, "sender1@example.com", []string{"rcpt@example.com"})

	if len(target2.messages) != 1 {
		t.Fatalf("wrong amount of messages received for target2, want %d, got %d", 1, len(target2.messages))
	}
	checkTestMessage(t, &target2, 0, "sender2@example.com", []string{"rcpt@example.com"})
}

func TestDispatcher_PerSourceReject(t *testing.T) {
	target := testTarget{}
	d := Dispatcher{
		dispatcherCfg: dispatcherCfg{
			perSource: map[string]sourceBlock{
				"sender1@example.com": {
					perRcpt: map[string]*rcptBlock{},
					defaultRcpt: &rcptBlock{
						targets: []module.DeliveryTarget{&target},
					},
				},
				"example.com": {perRcpt: map[string]*rcptBlock{},
					rejectErr: errors.New("go away"),
				},
			},
			defaultSource: sourceBlock{rejectErr: errors.New("go away")},
		},
		Log: testLogger(t, "dispatcher"),
	}

	doTestDelivery(t, &d, "sender1@example.com", []string{"rcpt@example.com"})

	_, err := d.Start(&module.DeliveryContext{DeliveryID: "testing"}, "sender2@example.com")
	if err == nil {
		t.Error("expected error for delivery.Start, got nil")
	}

	_, err = d.Start(&module.DeliveryContext{DeliveryID: "testing"}, "sender2@example.org")
	if err == nil {
		t.Error("expected error for delivery.Start, got nil")
	}
}

func TestDispatcher_PerRcptReject(t *testing.T) {
	target := testTarget{}
	d := Dispatcher{
		dispatcherCfg: dispatcherCfg{
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
		Log: testLogger(t, "dispatcher"),
	}

	delivery, err := d.Start(&module.DeliveryContext{DeliveryID: "testing"}, "sender@example.com")
	if err != nil {
		t.Fatalf("unexpected Start err: %v", err)
	}
	defer delivery.Abort()

	if err := delivery.AddRcpt("rcpt2@example.com"); err == nil {
		t.Fatalf("expected error for delivery.AddRcpt(rcpt2@example.com), got nil")
	}
	if err := delivery.AddRcpt("rcpt1@example.com"); err != nil {
		t.Fatalf("unexpected AddRcpt err for %s: %v", "rcpt1@example.com", err)
	}
	if err := delivery.Body(textproto.Header{}, buffer.MemoryBuffer{Slice: []byte("foobar")}); err != nil {
		t.Fatalf("unexpected Body err: %v", err)
	}
	if err := delivery.Commit(); err != nil {
		t.Fatalf("unexpected Commit err: %v", err)
	}
}

func TestDispatcher_PostmasterRcpt(t *testing.T) {
	target := testTarget{}
	d := Dispatcher{
		dispatcherCfg: dispatcherCfg{
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
		Log: testLogger(t, "dispatcher"),
	}

	doTestDelivery(t, &d, "disappointed-user@example.com", []string{"postmaster"})
	if len(target.messages) != 1 {
		t.Fatalf("wrong amount of messages received for target, want %d, got %d", 1, len(target.messages))
	}
	checkTestMessage(t, &target, 0, "disappointed-user@example.com", []string{"postmaster"})
}

func TestDispatcher_PostmasterSrc(t *testing.T) {
	target := testTarget{}
	d := Dispatcher{
		dispatcherCfg: dispatcherCfg{
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
		Log: testLogger(t, "dispatcher"),
	}

	doTestDelivery(t, &d, "postmaster", []string{"disappointed-user@example.com"})
	if len(target.messages) != 1 {
		t.Fatalf("wrong amount of messages received for target, want %d, got %d", 1, len(target.messages))
	}
	checkTestMessage(t, &target, 0, "postmaster", []string{"disappointed-user@example.com"})
}

func TestDispatcher_CaseInsensetiveMatch_Src(t *testing.T) {
	target := testTarget{}
	d := Dispatcher{
		dispatcherCfg: dispatcherCfg{
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
		Log: testLogger(t, "dispatcher"),
	}

	doTestDelivery(t, &d, "POSTMastER", []string{"disappointed-user@example.com"})
	doTestDelivery(t, &d, "SenDeR@EXAMPLE.com", []string{"disappointed-user@example.com"})
	doTestDelivery(t, &d, "sender@exAMPle.com", []string{"disappointed-user@example.com"})
	if len(target.messages) != 3 {
		t.Fatalf("wrong amount of messages received for target, want %d, got %d", 3, len(target.messages))
	}
	checkTestMessage(t, &target, 0, "POSTMastER", []string{"disappointed-user@example.com"})
	checkTestMessage(t, &target, 1, "SenDeR@EXAMPLE.com", []string{"disappointed-user@example.com"})
	checkTestMessage(t, &target, 2, "sender@exAMPle.com", []string{"disappointed-user@example.com"})
}

func TestDispatcher_CaseInsensetiveMatch_Rcpt(t *testing.T) {
	target := testTarget{}
	d := Dispatcher{
		dispatcherCfg: dispatcherCfg{
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
		Log: testLogger(t, "dispatcher"),
	}

	doTestDelivery(t, &d, "sender@example.com", []string{"POSTMastER"})
	doTestDelivery(t, &d, "sender@example.com", []string{"SenDeR@EXAMPLE.com"})
	doTestDelivery(t, &d, "sender@example.com", []string{"sender@exAMPle.com"})
	if len(target.messages) != 3 {
		t.Fatalf("wrong amount of messages received for target, want %d, got %d", 3, len(target.messages))
	}
	checkTestMessage(t, &target, 0, "sender@example.com", []string{"POSTMastER"})
	checkTestMessage(t, &target, 1, "sender@example.com", []string{"SenDeR@EXAMPLE.com"})
	checkTestMessage(t, &target, 2, "sender@example.com", []string{"sender@exAMPle.com"})
}

func TestDispatcher_MalformedSource(t *testing.T) {
	target := testTarget{}
	d := Dispatcher{
		dispatcherCfg: dispatcherCfg{
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
		Log: testLogger(t, "dispatcher"),
	}

	// Simple checks for violations that can make dispatcher misbehave.
	for _, addr := range []string{"not_postmaster_but_no_at_sign", "@no_mailbox", "no_domain@", "that@is@definiely@broken"} {
		_, err := d.Start(&module.DeliveryContext{DeliveryID: "testing"}, addr)
		if err == nil {
			t.Errorf("%s is accepted as valid address", addr)
		}
	}
}

func TestDispatcher_TwoRcptToOneTarget(t *testing.T) {
	target := testTarget{}
	d := Dispatcher{
		dispatcherCfg: dispatcherCfg{
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
		Log: testLogger(t, "dispatcher"),
	}

	doTestDelivery(t, &d, "sender@example.com", []string{"recipient@example.com", "recipient@example.org"})

	if len(target.messages) != 1 {
		t.Fatalf("wrong amount of messages received for target, want %d, got %d", 1, len(target.messages))
	}
	checkTestMessage(t, &target, 0, "sender@example.com", []string{"recipient@example.com", "recipient@example.org"})
}
