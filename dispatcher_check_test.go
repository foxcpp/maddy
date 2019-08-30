package maddy

import (
	"testing"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-msgauth/authres"
	"github.com/foxcpp/maddy/module"
)

func TestDispatcher_NoScoresChecked(t *testing.T) {
	target := testTarget{}
	check1, check2 := testCheck{
		bodyRes: module.CheckResult{ScoreAdjust: 5},
	}, testCheck{
		bodyRes: module.CheckResult{ScoreAdjust: 5},
	}
	d := Dispatcher{
		dispatcherCfg: dispatcherCfg{
			globalChecks: CheckGroup{checks: []module.Check{&check1, &check2}},
			perSource:    map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
		},
		Log: testLogger(t, "dispatcher"),
	}

	// No rejectScore or quarantineScore.
	doTestDelivery(t, &d, "whatever@whatever", []string{"whatever@whatever"})

	if len(target.messages) != 1 {
		t.Fatalf("wrong amount of messages received, want %d, got %d", 1, len(target.messages))
	}
	if target.messages[0].msgMeta.Quarantine {
		t.Fatalf("message is quarantined when it shouldn't")
	}
}

func TestDispatcher_RejectScore(t *testing.T) {
	target := testTarget{}
	check1, check2 := testCheck{
		bodyRes: module.CheckResult{ScoreAdjust: 5},
	}, testCheck{
		bodyRes: module.CheckResult{ScoreAdjust: 5},
	}
	rejectScore := 10
	d := Dispatcher{
		dispatcherCfg: dispatcherCfg{
			globalChecks: CheckGroup{checks: []module.Check{&check1, &check2}},
			perSource:    map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
			rejectScore: &rejectScore,
		},
		Log: testLogger(t, "dispatcher"),
	}

	// Should be rejected.
	if _, err := doTestDeliveryErr(t, &d, "whatever@whatever", []string{"whatever@whatever"}); err == nil {
		t.Fatalf("expected an error")
	}

	if len(target.messages) != 0 {
		t.Fatalf("wrong amount of messages received, want %d, got %d", 0, len(target.messages))
	}
}

func TestDispatcher_RejectScore_notEnough(t *testing.T) {
	target := testTarget{}
	check1, check2 := testCheck{
		bodyRes: module.CheckResult{ScoreAdjust: 5},
	}, testCheck{
		bodyRes: module.CheckResult{ScoreAdjust: 5},
	}
	rejectScore := 15
	d := Dispatcher{
		dispatcherCfg: dispatcherCfg{
			globalChecks: CheckGroup{checks: []module.Check{&check1, &check2}},
			perSource:    map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
			rejectScore: &rejectScore,
		},
		Log: testLogger(t, "dispatcher"),
	}

	doTestDelivery(t, &d, "whatever@whatever", []string{"whatever@whatever"})

	if len(target.messages) != 1 {
		t.Fatalf("wrong amount of messages received, want %d, got %d", 1, len(target.messages))
	}
	if target.messages[0].msgMeta.Quarantine {
		t.Fatalf("message is quarantined when it shouldn't")
	}
}

func TestDispatcher_Quarantine(t *testing.T) {
	target := testTarget{}
	check1, check2 := testCheck{
		bodyRes: module.CheckResult{},
	}, testCheck{
		bodyRes: module.CheckResult{Quarantine: true},
	}
	d := Dispatcher{
		dispatcherCfg: dispatcherCfg{
			globalChecks: CheckGroup{checks: []module.Check{&check1, &check2}},
			perSource:    map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
		},
		Log: testLogger(t, "dispatcher"),
	}

	// Should be quarantined.
	doTestDelivery(t, &d, "whatever@whatever", []string{"whatever@whatever"})

	if len(target.messages) != 1 {
		t.Fatalf("wrong amount of messages received, want %d, got %d", 1, len(target.messages))
	}
	if !target.messages[0].msgMeta.Quarantine {
		t.Fatalf("message is not quarantined when it should")
	}
}

func TestDispatcher_QuarantineScore(t *testing.T) {
	target := testTarget{}
	check1, check2 := testCheck{
		bodyRes: module.CheckResult{ScoreAdjust: 5},
	}, testCheck{
		bodyRes: module.CheckResult{ScoreAdjust: 5},
	}
	quarantineScore := 10
	d := Dispatcher{
		dispatcherCfg: dispatcherCfg{
			globalChecks: CheckGroup{checks: []module.Check{&check1, &check2}},
			perSource:    map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
			quarantineScore: &quarantineScore,
		},
		Log: testLogger(t, "dispatcher"),
	}

	// Should be quarantined.
	doTestDelivery(t, &d, "whatever@whatever", []string{"whatever@whatever"})

	if len(target.messages) != 1 {
		t.Fatalf("wrong amount of messages received, want %d, got %d", 1, len(target.messages))
	}
	if !target.messages[0].msgMeta.Quarantine {
		t.Fatalf("message is not quarantined when it should")
	}
}

func TestDispatcher_QuarantineScore_notEnough(t *testing.T) {
	target := testTarget{}
	check1, check2 := testCheck{
		bodyRes: module.CheckResult{ScoreAdjust: 5},
	}, testCheck{
		bodyRes: module.CheckResult{ScoreAdjust: 5},
	}
	quarantineScore := 15
	d := Dispatcher{
		dispatcherCfg: dispatcherCfg{
			globalChecks: CheckGroup{checks: []module.Check{&check1, &check2}},
			perSource:    map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
			quarantineScore: &quarantineScore,
		},
		Log: testLogger(t, "dispatcher"),
	}

	// Should be quarantined.
	doTestDelivery(t, &d, "whatever@whatever", []string{"whatever@whatever"})

	if len(target.messages) != 1 {
		t.Fatalf("wrong amount of messages received, want %d, got %d", 1, len(target.messages))
	}
	if target.messages[0].msgMeta.Quarantine {
		t.Fatalf("message is quarantined when it shouldn't")
	}
}

func TestDispatcher_BothScores_Quarantined(t *testing.T) {
	target := testTarget{}
	check1, check2 := testCheck{
		bodyRes: module.CheckResult{ScoreAdjust: 5},
	}, testCheck{
		bodyRes: module.CheckResult{ScoreAdjust: 5},
	}
	quarantineScore := 10
	rejectScore := 15
	d := Dispatcher{
		dispatcherCfg: dispatcherCfg{
			globalChecks: CheckGroup{checks: []module.Check{&check1, &check2}},
			perSource:    map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
			quarantineScore: &quarantineScore,
			rejectScore:     &rejectScore,
		},
		Log: testLogger(t, "dispatcher"),
	}

	// Should be quarantined.
	doTestDelivery(t, &d, "whatever@whatever", []string{"whatever@whatever"})

	if len(target.messages) != 1 {
		t.Fatalf("wrong amount of messages received, want %d, got %d", 1, len(target.messages))
	}
	if !target.messages[0].msgMeta.Quarantine {
		t.Fatalf("message is not quarantined when it should")
	}
}

func TestDispatcher_BothScores_Rejected(t *testing.T) {
	target := testTarget{}
	check1, check2 := testCheck{
		bodyRes: module.CheckResult{ScoreAdjust: 5},
	}, testCheck{
		bodyRes: module.CheckResult{ScoreAdjust: 5},
	}
	quarantineScore := 5
	rejectScore := 10
	d := Dispatcher{
		dispatcherCfg: dispatcherCfg{
			globalChecks: CheckGroup{checks: []module.Check{&check1, &check2}},
			perSource:    map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
			quarantineScore: &quarantineScore,
			rejectScore:     &rejectScore,
		},
		Log: testLogger(t, "dispatcher"),
	}

	// Should be quarantined.
	if _, err := doTestDeliveryErr(t, &d, "whatever@whatever", []string{"whatever@whatever"}); err == nil {
		t.Fatalf("message not rejected")
	}

	if len(target.messages) != 0 {
		t.Fatalf("wrong amount of messages received, want %d, got %d", 0, len(target.messages))
	}
}

func TestDispatcher_AuthResults(t *testing.T) {
	target := testTarget{}
	check1, check2 := testCheck{
		bodyRes: module.CheckResult{
			AuthResult: []authres.Result{
				&authres.SPFResult{
					Value: authres.ResultFail,
					From:  "FROM",
					Helo:  "HELO",
				},
			},
		},
	}, testCheck{
		bodyRes: module.CheckResult{
			AuthResult: []authres.Result{
				&authres.SPFResult{
					Value: authres.ResultFail,
					From:  "FROM2",
					Helo:  "HELO2",
				},
			},
		},
	}
	d := Dispatcher{
		dispatcherCfg: dispatcherCfg{
			globalChecks: CheckGroup{checks: []module.Check{&check1, &check2}},
			perSource:    map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
		},
		hostname: "TEST-HOST",
		Log:      testLogger(t, "dispatcher"),
	}

	doTestDelivery(t, &d, "whatever@whatever", []string{"whatever@whatever"})

	if len(target.messages) != 1 {
		t.Fatalf("wrong amount of messages received, want %d, got %d", 1, len(target.messages))
	}

	authRes := target.messages[0].header.Get("Authentication-Results")
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
}

func TestDispatcher_Headers(t *testing.T) {
	hdr1 := textproto.Header{}
	hdr1.Add("HDR1", "1")
	hdr2 := textproto.Header{}
	hdr2.Add("HDR2", "2")

	target := testTarget{}
	check1, check2 := testCheck{
		bodyRes: module.CheckResult{
			Header: hdr1,
		},
	}, testCheck{
		bodyRes: module.CheckResult{
			Header: hdr2,
		},
	}
	d := Dispatcher{
		dispatcherCfg: dispatcherCfg{
			globalChecks: CheckGroup{checks: []module.Check{&check1, &check2}},
			perSource:    map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
		},
		hostname: "TEST-HOST",
		Log:      testLogger(t, "dispatcher"),
	}

	doTestDelivery(t, &d, "whatever@whatever", []string{"whatever@whatever"})

	if len(target.messages) != 1 {
		t.Fatalf("wrong amount of messages received, want %d, got %d", 1, len(target.messages))
	}

	if target.messages[0].header.Get("HDR1") != "1" {
		t.Fatalf("wrong HDR1 value, want %s, got %s", "1", target.messages[0].header.Get("HDR1"))
	}
	if target.messages[0].header.Get("HDR2") != "2" {
		t.Fatalf("wrong HDR2 value, want %s, got %s", "1", target.messages[0].header.Get("HDR2"))
	}
}
