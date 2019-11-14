package msgpipeline

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-msgauth/authres"
	"github.com/foxcpp/go-mockdns"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/module"
	"github.com/foxcpp/maddy/testutils"
)

func doTestDelivery(t *testing.T, tgt module.DeliveryTarget, from string, to []string, hdr string) (string, error) {
	t.Helper()

	IDRaw := sha1.Sum([]byte(t.Name()))
	encodedID := hex.EncodeToString(IDRaw[:])

	body := buffer.MemoryBuffer{Slice: []byte("foobar")}
	ctx := module.MsgMetadata{
		DontTraceSender: true,
		ID:              encodedID,
	}

	hdrParsed, err := textproto.ReadHeader(bufio.NewReader(strings.NewReader(hdr)))
	if err != nil {
		panic(err)
	}

	delivery, err := tgt.Start(&ctx, from)
	if err != nil {
		return encodedID, err
	}
	for _, rcpt := range to {
		if err := delivery.AddRcpt(rcpt); err != nil {
			if err := delivery.Abort(); err != nil {
			}
			return encodedID, err
		}
	}
	if err := delivery.Body(hdrParsed, body); err != nil {
		if err := delivery.Abort(); err != nil {
			t.Log("delivery.Abort:", err)
		}
		return encodedID, err
	}
	if err := delivery.Commit(); err != nil {
		return encodedID, err
	}

	return encodedID, err
}

func dmarcResult(t *testing.T, hdr textproto.Header) authres.ResultValue {
	field := hdr.Get("Authentication-Results")
	if field == "" {
		t.Fatalf("No results field")
	}

	_, results, err := authres.Parse(field)
	if err != nil {
		t.Fatalf("Field parse err: %v", err)
	}

	for _, res := range results {
		dmarcRes, ok := res.(*authres.DMARCResult)
		if ok {
			return dmarcRes.Value
		}
	}

	t.Fatalf("No DMARC authres found")
	return ""
}

func TestDMARC(t *testing.T) {
	test := func(zones map[string]mockdns.Zone, hdr string, authres []authres.Result, reject, quarantine bool, dmarcRes authres.ResultValue) {
		t.Helper()

		tgt := testutils.Target{}
		p := MsgPipeline{
			msgpipelineCfg: msgpipelineCfg{
				globalChecks: []module.Check{
					&testutils.Check{
						BodyRes: module.CheckResult{
							AuthResult: authres,
						},
					},
				},
				perSource: map[string]sourceBlock{},
				defaultSource: sourceBlock{
					perRcpt: map[string]*rcptBlock{},
					defaultRcpt: &rcptBlock{
						targets: []module.DeliveryTarget{&tgt},
					},
				},
				doDMARC: true,
			},
			Log:      testutils.Logger(t, "pipeline"),
			Resolver: &mockdns.Resolver{Zones: zones},
		}

		_, err := doTestDelivery(t, &p, "test@example.org", []string{"test@example.com"}, hdr)
		if reject {
			if err == nil {
				t.Errorf("expected message to be rejected")
				return
			}
			return
		}
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		if len(tgt.Messages) != 1 {
			t.Errorf("got %d messages", len(tgt.Messages))
			return
		}
		msg := tgt.Messages[0]

		if msg.MsgMeta.Quarantine != quarantine {
			t.Errorf("msg.MsgMeta.Quarantine (%v) != quarantine (%v)", msg.MsgMeta.Quarantine, quarantine)
			return
		}

		res := dmarcResult(t, msg.Header)
		if res != dmarcRes {
			t.Errorf("expected DMARC result to be '%v', got '%v'", dmarcRes, res)
			return
		}
	}

	// No policy => DMARC 'none'
	test(map[string]mockdns.Zone{}, "From: hello@example.org\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
	}, false, false, authres.ResultNone)

	// Policy present & identifiers align => DMARC 'pass'
	test(map[string]mockdns.Zone{
		"_dmarc.example.org.": mockdns.Zone{
			TXT: []string{"v=DMARC1; p=none"},
		},
	}, "From: hello@example.org\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
	}, false, false, authres.ResultPass)

	// Check org. domain and from domain, prefer from domain.
	// https://tools.ietf.org/html/rfc7489#section-6.6.3
	test(map[string]mockdns.Zone{
		"_dmarc.example.org.": mockdns.Zone{
			TXT: []string{"v=DMARC1; p=none"},
		},
	}, "From: hello@sub.example.org\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
	}, false, false, authres.ResultPass)
	test(map[string]mockdns.Zone{
		"_dmarc.sub.example.org.": mockdns.Zone{
			TXT: []string{"v=DMARC1; p=none"},
		},
	}, "From: hello@sub.example.org\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
	}, false, false, authres.ResultPass)
	test(map[string]mockdns.Zone{
		"_dmarc.sub.example.org.": mockdns.Zone{
			TXT: []string{"v=DMARC1; p=none"},
		},
		"_dmarc.example.org.": mockdns.Zone{
			TXT: []string{"v=malformed"},
		},
	}, "From: hello@sub.example.org\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
	}, false, false, authres.ResultPass)

	// Non-DMARC records are ignored.
	// https://tools.ietf.org/html/rfc7489#section-6.6.3
	test(map[string]mockdns.Zone{
		"_dmarc.example.org.": mockdns.Zone{
			TXT: []string{"ignore", "v=DMARC1; p=none"},
		},
	}, "From: hello@sub.example.org\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
	}, false, false, authres.ResultPass)

	// Multiple policies => no policy.
	// https://tools.ietf.org/html/rfc7489#section-6.6.3
	test(map[string]mockdns.Zone{
		"_dmarc.example.org.": mockdns.Zone{
			TXT: []string{"v=DMARC1; p=reject", "v=DMARC1; p=none"},
		},
	}, "From: hello@sub.example.org\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
	}, false, false, authres.ResultNone)

	// Malformed policy => no policy
	test(map[string]mockdns.Zone{
		"_dmarc.example.com.": mockdns.Zone{
			TXT: []string{"v=aaaa"},
		},
	}, "From: hello@example.com\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
	}, false, false, authres.ResultNone)

	// Policy fetch error => DMARC 'permerror' but the message
	// is accepted.
	test(map[string]mockdns.Zone{
		"_dmarc.example.com.": mockdns.Zone{
			Err: errors.New("the dns server is going insane"),
		},
	}, "From: hello@example.com\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
	}, false, false, authres.ResultPermError)

	// Policy fetch error => DMARC 'temperror' but the message
	// is accepted ("fail open")
	test(map[string]mockdns.Zone{
		"_dmarc.example.com.": mockdns.Zone{
			Err: &net.DNSError{
				Err:         "the dns server is going insane, temporarly",
				IsTemporary: true,
			},
		},
	}, "From: hello@example.com\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
	}, false, false, authres.ResultTempError)

	// Misaligned From vs DKIM => DMARC 'fail'.
	// Side note: More comprehensive tests for alignment evaluation
	// can be found in check/dmarc/evaluate_test.go. This test merely checks
	// that the correct action is taken based on the policy.
	test(map[string]mockdns.Zone{
		"_dmarc.example.com.": mockdns.Zone{
			TXT: []string{"v=DMARC1; p=none"},
		},
	}, "From: hello@example.com\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
	}, false, false, authres.ResultFail)

	// Misaligned From vs DKIM => DMARC 'fail', policy says to reject
	test(map[string]mockdns.Zone{
		"_dmarc.example.com.": mockdns.Zone{
			TXT: []string{"v=DMARC1; p=reject"},
		},
	}, "From: hello@example.com\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
	}, true, false, "")

	// Misaligned From vs DKIM => DMARC 'fail'
	// Subdomain policy requests no action, main domain policy says to reject.
	test(map[string]mockdns.Zone{
		"_dmarc.example.com.": mockdns.Zone{
			TXT: []string{"v=DMARC1; sp=none; p=reject"},
		},
	}, "From: hello@sub.example.com\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
	}, false, false, authres.ResultFail)

	// Misaligned From vs DKIM => DMARC 'fail', policy says to quarantine.
	test(map[string]mockdns.Zone{
		"_dmarc.example.com.": mockdns.Zone{
			TXT: []string{"v=DMARC1; p=quarantine"},
		},
	}, "From: hello@example.com\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
	}, false, true, authres.ResultFail)
}
