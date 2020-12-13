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

package dkim

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/emersion/go-msgauth/authres"
	"github.com/foxcpp/go-mockdns"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/testutils"
)

const unsignedMailString = `From: Joe SixPack <joe@football.example.com>
To: Suzie Q <suzie@shopping.example.net>
Subject: Is dinner ready?
Date: Fri, 11 Jul 2003 21:00:37 -0700 (PDT)
Message-ID: <20030712040037.46341.5F8J@football.example.com>

Hi.

We lost the game. Are you hungry yet?

Joe.
`

const dnsPublicKey = "v=DKIM1; p=MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQ" +
	"KBgQDwIRP/UC3SBsEmGqZ9ZJW3/DkMoGeLnQg1fWn7/zYt" +
	"IxN2SnFCjxOCKG9v3b4jYfcTNh5ijSsq631uBItLa7od+v" +
	"/RtdC2UzJ1lWT947qR+Rcac2gbto/NMqJ0fzfVjH4OuKhi" +
	"tdY9tf6mcwGjaNBcWToIMmPSPDdQPNUYckcQ2QIDAQAB"

var testZones = map[string]mockdns.Zone{
	"brisbane._domainkey.example.com.": {
		TXT: []string{dnsPublicKey},
	},
}

const verifiedMailString = `DKIM-Signature: v=1; a=rsa-sha256; s=brisbane; d=example.com;
      c=simple/simple; q=dns/txt; i=joe@football.example.com;
      h=Received : From : To : Subject : Date : Message-ID;
      bh=2jUSOH9NhtVGCQWNr9BrIAPreKQjO6Sn7XIkfJVOzv8=;
      b=AuUoFEfDxTDkHlLXSZEpZj79LICEps6eda7W3deTVFOk4yAUoqOB
      4nujc7YopdG5dWLSdNg6xNAZpOPr+kHxt1IrE+NahM6L/LbvaHut
      KVdkLLkpVaVVQPzeRDI009SO2Il5Lu7rDNH6mZckBdrIx0orEtZV
      4bmp/YzhwvcubU4=;
Received: from client1.football.example.com  [192.0.2.1]
      by submitserver.example.com with SUBMISSION;
      Fri, 11 Jul 2003 21:01:54 -0700 (PDT)
From: Joe SixPack <joe@football.example.com>
To: Suzie Q <suzie@shopping.example.net>
Subject: Is dinner ready?
Date: Fri, 11 Jul 2003 21:00:37 -0700 (PDT)
Message-ID: <20030712040037.46341.5F8J@football.example.com>

Hi.

We lost the game. Are you hungry yet?

Joe.
`

func testCheck(t *testing.T, zones map[string]mockdns.Zone, cfg []config.Node) *Check {
	t.Helper()
	mod, err := New("check.dkim", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	check := mod.(*Check)
	check.resolver = &mockdns.Resolver{Zones: zones}
	check.log = testutils.Logger(t, mod.Name())

	if err := check.Init(config.NewMap(nil, config.Node{Children: cfg})); err != nil {
		t.Fatal(err)
	}

	return check
}

func TestDkimVerify_NoSig(t *testing.T) {
	check := testCheck(t, nil, nil) // No zones since this test requires no lookups.

	// Force certain reason so we can assert for it.
	check.noSigAction.Reject = true
	check.noSigAction.ReasonOverride = &exterrors.SMTPError{Code: 555}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// The usual checking flow.
	s, err := check.CheckStateForMsg(ctx, &module.MsgMetadata{
		ID: "test_unsigned",
	})
	if err != nil {
		t.Fatal(err)
	}
	s.CheckConnection(ctx)
	s.CheckSender(ctx, "joe@football.example.com")
	s.CheckRcpt(ctx, "suzie@shopping.example.net")

	hdr, buf := testutils.BodyFromStr(t, unsignedMailString)
	result := s.CheckBody(ctx, hdr, buf)

	if result.Reason == nil {
		t.Fatal("No check fail reason set, auth. result:", authres.Format("", result.AuthResult))
	}
	if result.Reason.(*exterrors.SMTPError).Code != 555 {
		t.Fatal("Different fail reason:", result.Reason)
	}
}

func TestDkimVerify_InvalidSig(t *testing.T) {
	check := testCheck(t, testZones, nil)

	// Force certain reason so we can assert for it.
	check.brokenSigAction.Reject = true
	check.brokenSigAction.ReasonOverride = &exterrors.SMTPError{Code: 555}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := check.CheckStateForMsg(ctx, &module.MsgMetadata{
		ID: "test_unsigned",
	})
	if err != nil {
		t.Fatal(err)
	}

	s.CheckConnection(ctx)
	s.CheckSender(ctx, "joe@football.example.com")
	s.CheckRcpt(ctx, "suzie@shopping.example.net")

	hdr, buf := testutils.BodyFromStr(t, verifiedMailString)
	// Mess up the signature.
	hdr.Set("From", "nope")

	result := s.CheckBody(ctx, hdr, buf)

	if result.Reason == nil {
		t.Fatal("No check fail reason set, auth. result:", authres.Format("", result.AuthResult))
	}
	if result.Reason.(*exterrors.SMTPError).Code != 555 {
		t.Fatal("Different fail reason:", result.Reason)
	}
}

func TestDkimVerify_ValidSig(t *testing.T) {
	check := testCheck(t, testZones, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := check.CheckStateForMsg(ctx, &module.MsgMetadata{
		ID: "test_unsigned",
	})
	if err != nil {
		t.Fatal(err)
	}

	s.CheckConnection(ctx)
	s.CheckSender(ctx, "joe@football.example.com")
	s.CheckRcpt(ctx, "suzie@shopping.example.net")

	hdr, buf := testutils.BodyFromStr(t, verifiedMailString)

	result := s.CheckBody(ctx, hdr, buf)

	if result.Reason != nil {
		t.Log(authres.Format("", result.AuthResult))
		t.Fatal("Check fail reason set, auth. result:", result.Reason, exterrors.Fields(result.Reason))
	}
}

func TestDkimVerify_RequiredFields(t *testing.T) {
	check := testCheck(t, testZones, []config.Node{
		{
			// Require field that is not covered by the signature.
			Name: "required_fields",
			Args: []string{"From", "X-Important"},
		},
	})

	// Force certain reason so we can assert for it.
	check.brokenSigAction.Reject = true
	check.brokenSigAction.ReasonOverride = &exterrors.SMTPError{Code: 555}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := check.CheckStateForMsg(ctx, &module.MsgMetadata{
		ID: "test_unsigned",
	})
	if err != nil {
		t.Fatal(err)
	}

	s.CheckConnection(ctx)
	s.CheckSender(ctx, "joe@football.example.com")
	s.CheckRcpt(ctx, "suzie@shopping.example.net")

	hdr, buf := testutils.BodyFromStr(t, verifiedMailString)

	result := s.CheckBody(ctx, hdr, buf)

	if result.Reason == nil {
		t.Fatal("No check fail reason set, auth. result:", authres.Format("", result.AuthResult))
	}
	if result.Reason.(*exterrors.SMTPError).Code != 555 {
		t.Fatal("Different fail reason:", result.Reason)
	}
}

func TestDkimVerify_BufferOpenFail(t *testing.T) {
	check := testCheck(t, testZones, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := check.CheckStateForMsg(ctx, &module.MsgMetadata{
		ID: "test_unsigned",
	})
	if err != nil {
		t.Fatal(err)
	}

	s.CheckConnection(ctx)
	s.CheckSender(ctx, "joe@football.example.com")
	s.CheckRcpt(ctx, "suzie@shopping.example.net")

	var buf buffer.Buffer
	hdr, buf := testutils.BodyFromStr(t, verifiedMailString)
	buf = testutils.FailingBuffer{Blob: buf.(buffer.MemoryBuffer).Slice, OpenError: errors.New("No!")}

	result := s.CheckBody(ctx, hdr, buf)
	t.Log("auth. result:", authres.Format("", result.AuthResult))

	if result.Reason == nil {
		t.Fatal("No check fail reason set, auth. result:", authres.Format("", result.AuthResult))
	}
}

func TestDkimVerify_FailClosed(t *testing.T) {
	zones := map[string]mockdns.Zone{
		"brisbane._domainkey.example.com.": {
			Err: &net.DNSError{
				Err:         "DNS server is not having a great time",
				IsTemporary: true,
				IsTimeout:   true,
			},
		},
	}
	check := testCheck(t, zones, []config.Node{
		{
			Name: "fail_open",
			Args: []string{"false"},
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := check.CheckStateForMsg(ctx, &module.MsgMetadata{
		ID: "test_unsigned",
	})
	if err != nil {
		t.Fatal(err)
	}

	s.CheckConnection(ctx)
	s.CheckSender(ctx, "joe@football.example.com")
	s.CheckRcpt(ctx, "suzie@shopping.example.net")

	hdr, buf := testutils.BodyFromStr(t, verifiedMailString)

	result := s.CheckBody(ctx, hdr, buf)
	t.Log("auth. result:", authres.Format("", result.AuthResult))

	if result.Reason == nil {
		t.Fatal("No check fail reason set, auth. result:", authres.Format("", result.AuthResult))
	}
	if !result.Reject {
		t.Fatal("No reject requested")
	}
	if !exterrors.IsTemporary(result.Reason) {
		t.Fatal("Fail reason is not marked as temporary:", result.Reason)
	}
}

func TestDkimVerify_FailOpen(t *testing.T) {
	zones := map[string]mockdns.Zone{
		"brisbane._domainkey.example.com.": {
			Err: &net.DNSError{
				Err:         "DNS server is not having a great time",
				IsTemporary: true,
				IsTimeout:   true,
			},
		},
	}
	check := testCheck(t, zones, []config.Node{
		{
			Name: "fail_open",
			Args: []string{"true"},
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := check.CheckStateForMsg(ctx, &module.MsgMetadata{
		ID: "test_unsigned",
	})
	if err != nil {
		t.Fatal(err)
	}

	s.CheckConnection(ctx)
	s.CheckSender(ctx, "joe@football.example.com")
	s.CheckRcpt(ctx, "suzie@shopping.example.net")

	hdr, buf := testutils.BodyFromStr(t, verifiedMailString)

	result := s.CheckBody(ctx, hdr, buf)

	t.Log("auth. result:", authres.Format("", result.AuthResult))
	if result.Reason == nil {
		t.Fatal("No check fail reason set, auth. result:", authres.Format("", result.AuthResult))
	}
	if result.Reject {
		t.Fatal("Reject requested")
	}
	if exterrors.IsTemporary(result.Reason) {
		t.Fatal("Fail reason is not marked as temporary:", result.Reason)
	}

	if len(result.AuthResult) != 1 {
		t.Fatal("Wrong amount of auth. result fields:", len(result.AuthResult))
	}
	resVal := result.AuthResult[0].(*authres.DKIMResult).Value
	if resVal != authres.ResultTempError {
		t.Fatal("Result is not temp. error:", resVal)
	}
}
