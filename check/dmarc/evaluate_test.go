package dmarc

import (
	"bufio"
	"strings"
	"testing"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-msgauth/authres"
	"github.com/emersion/go-msgauth/dmarc"
)

func TestEvaluateAlignment(t *testing.T) {
	type tCase struct {
		orgDomain string
		record    *dmarc.Record
		results   []authres.Result

		output authres.ResultValue
	}
	test := func(i int, c tCase) {
		out := EvaluateAlignment(c.orgDomain, c.record, c.results)
		if out.Value != c.output {
			t.Errorf("%d: Wrong eval result, want '%s', got '%s' (%s)", i, c.output, out.Value, out.Reason)
		}
	}

	cases := []tCase{
		{ // 0
			orgDomain: "example.org",
			record:    &dmarc.Record{},

			output: authres.ResultNone,
		},
		{ // 1
			orgDomain: "example.org",
			record:    &dmarc.Record{},
			results: []authres.Result{
				&authres.SPFResult{
					Value: authres.ResultFail,
					From:  "example.org",
					Helo:  "mx.example.org",
				},
				&authres.DKIMResult{
					Value:  authres.ResultNone,
					Domain: "example.org",
				},
			},
			output: authres.ResultFail,
		},
		{ // 2
			orgDomain: "example.org",
			record:    &dmarc.Record{},
			results: []authres.Result{
				&authres.SPFResult{
					Value: authres.ResultPass,
					From:  "example.org",
					Helo:  "mx.example.org",
				},
				&authres.DKIMResult{
					Value:  authres.ResultNone,
					Domain: "example.org",
				},
			},
			output: authres.ResultPass,
		},
		{ // 3
			orgDomain: "example.org",
			record:    &dmarc.Record{},
			results: []authres.Result{
				&authres.SPFResult{
					Value: authres.ResultFail,
					From:  "example.org",
					Helo:  "mx.example.org",
				},
				&authres.DKIMResult{
					Value:  authres.ResultNone,
					Domain: "example.org",
				},
			},
			output: authres.ResultFail,
		},
		{ // 4
			orgDomain: "example.org",
			record:    &dmarc.Record{},
			results: []authres.Result{
				&authres.SPFResult{
					Value: authres.ResultPass,
					From:  "example.com",
					Helo:  "mx.example.com",
				},
				&authres.DKIMResult{
					Value:  authres.ResultNone,
					Domain: "example.org",
				},
			},
			output: authres.ResultFail,
		},
		{ // 5
			orgDomain: "example.com",
			record:    &dmarc.Record{},
			results: []authres.Result{
				&authres.SPFResult{
					Value: authres.ResultPass,
					From:  "cbg.bounces.example.com",
					Helo:  "mx.example.com",
				},
				&authres.DKIMResult{
					Value:  authres.ResultNone,
					Domain: "example.org",
				},
			},
			output: authres.ResultPass,
		},
		{ // 6
			orgDomain: "example.com",
			record: &dmarc.Record{
				SPFAlignment: dmarc.AlignmentStrict,
			},
			results: []authres.Result{
				&authres.SPFResult{
					Value: authres.ResultPass,
					From:  "cbg.bounces.example.com",
					Helo:  "mx.example.com",
				},
				&authres.DKIMResult{
					Value:  authres.ResultNone,
					Domain: "example.org",
				},
			},
			output: authres.ResultFail,
		},
		{ // 7
			orgDomain: "example.org",
			record:    &dmarc.Record{},
			results: []authres.Result{
				&authres.DKIMResult{
					Value:  authres.ResultFail,
					Domain: "example.org",
				},
				&authres.SPFResult{
					Value: authres.ResultNone,
					From:  "example.org",
					Helo:  "mx.example.org",
				},
			},
			output: authres.ResultFail,
		},
		{ // 8
			orgDomain: "example.org",
			record:    &dmarc.Record{},
			results: []authres.Result{
				&authres.DKIMResult{
					Value:  authres.ResultPass,
					Domain: "example.org",
				},
				&authres.SPFResult{
					Value: authres.ResultNone,
					From:  "example.org",
					Helo:  "mx.example.org",
				},
			},
			output: authres.ResultPass,
		},
		{ // 9
			orgDomain: "example.com",
			record:    &dmarc.Record{},
			results: []authres.Result{
				&authres.SPFResult{
					Value: authres.ResultPass,
					From:  "cbg.bounces.example.com",
					Helo:  "mx.example.com",
				},
				&authres.DKIMResult{
					Value:  authres.ResultPass,
					Domain: "example.com",
				},
			},
			output: authres.ResultPass,
		},
		{ // 10
			orgDomain: "example.com",
			record: &dmarc.Record{
				SPFAlignment: dmarc.AlignmentRelaxed,
			},
			results: []authres.Result{
				&authres.SPFResult{
					Value: authres.ResultPass,
					From:  "cbg.bounces.example.com",
					Helo:  "mx.example.com",
				},
				&authres.DKIMResult{
					Value:  authres.ResultFail,
					Domain: "example.com",
				},
			},
			output: authres.ResultPass,
		},
		{ // 11
			orgDomain: "example.com",
			record: &dmarc.Record{
				SPFAlignment: dmarc.AlignmentStrict,
			},
			results: []authres.Result{
				&authres.SPFResult{
					Value: authres.ResultPass,
					From:  "cbg.bounces.example.com",
					Helo:  "mx.example.com",
				},
				&authres.DKIMResult{
					Value:  authres.ResultPass,
					Domain: "example.com",
				},
			},
			output: authres.ResultPass,
		},
		{ // 12
			orgDomain: "example.com",
			record: &dmarc.Record{
				SPFAlignment:  dmarc.AlignmentStrict,
				DKIMAlignment: dmarc.AlignmentStrict,
			},
			results: []authres.Result{
				&authres.SPFResult{
					Value: authres.ResultPass,
					From:  "cbg.bounces.example.com",
					Helo:  "mx.example.com",
				},
				&authres.DKIMResult{
					Value:  authres.ResultFail,
					Domain: "cbg.example.com",
				},
			},
			output: authres.ResultFail,
		},
		{ // 13
			orgDomain: "example.org",
			record:    &dmarc.Record{},
			results: []authres.Result{
				&authres.DKIMResult{
					Value:  authres.ResultFail,
					Domain: "example.org",
				},
				&authres.DKIMResult{
					Value:  authres.ResultPass,
					Domain: "example.net",
				},
				&authres.DKIMResult{
					Value:  authres.ResultPass,
					Domain: "example.org",
				},
				&authres.DKIMResult{
					Value:  authres.ResultFail,
					Domain: "example.com",
				},
				&authres.SPFResult{
					Value: authres.ResultNone,
					From:  "example.org",
					Helo:  "mx.example.org",
				},
			},
			output: authres.ResultPass,
		},
		{ // 14
			orgDomain: "example.com",
			record:    &dmarc.Record{},
			results: []authres.Result{
				&authres.SPFResult{
					Value: authres.ResultPass,
					From:  "",
					Helo:  "mx.example.com",
				},
				&authres.DKIMResult{
					Value:  authres.ResultNone,
					Domain: "example.org",
				},
			},
			output: authres.ResultPass,
		},
		{ // 15
			orgDomain: "example.com",
			record: &dmarc.Record{
				SPFAlignment: dmarc.AlignmentStrict,
			},
			results: []authres.Result{
				&authres.SPFResult{
					Value: authres.ResultPass,
					From:  "",
					Helo:  "mx.example.com",
				},
				&authres.DKIMResult{
					Value:  authres.ResultNone,
					Domain: "example.org",
				},
			},
			output: authres.ResultFail,
		},
		{ // 16
			orgDomain: "example.com",
			record: &dmarc.Record{
				SPFAlignment: dmarc.AlignmentStrict,
			},
			results: []authres.Result{
				&authres.SPFResult{
					Value: authres.ResultPass,
					From:  "",
					Helo:  "mx.example.com",
				},
				&authres.DKIMResult{
					Value:  authres.ResultNone,
					Domain: "example.org",
				},
			},
			output: authres.ResultFail,
		},
		{ // 17
			orgDomain: "example.com",
			record:    &dmarc.Record{},
			results: []authres.Result{
				&authres.SPFResult{
					Value: authres.ResultTempError,
					From:  "",
					Helo:  "mx.example.com",
				},
				&authres.DKIMResult{
					Value:  authres.ResultNone,
					Domain: "example.org",
				},
			},
			output: authres.ResultTempError,
		},
		{ // 18
			orgDomain: "example.com",
			record:    &dmarc.Record{},
			results: []authres.Result{
				&authres.DKIMResult{
					Value:  authres.ResultTempError,
					Domain: "example.com",
				},
				&authres.SPFResult{
					Value: authres.ResultNone,
					From:  "example.org",
					Helo:  "mx.example.org",
				},
			},
			output: authres.ResultTempError,
		},
		{ // 19
			orgDomain: "example.com",
			record:    &dmarc.Record{},
			results: []authres.Result{
				&authres.SPFResult{
					Value: authres.ResultTempError,
					From:  "",
					Helo:  "mx.example.com",
				},
				&authres.DKIMResult{
					Value:  authres.ResultPass,
					Domain: "example.com",
				},
			},
			output: authres.ResultPass,
		},
		{ // 20
			orgDomain: "example.com",
			record:    &dmarc.Record{},
			results: []authres.Result{
				&authres.SPFResult{
					Value: authres.ResultPass,
					From:  "",
					Helo:  "mx.example.com",
				},
				&authres.DKIMResult{
					Value:  authres.ResultTempError,
					Domain: "example.com",
				},
			},
			output: authres.ResultPass,
		},
		{ // 21
			orgDomain: "example.org",
			record:    &dmarc.Record{},
			results: []authres.Result{
				&authres.DKIMResult{
					Value:  authres.ResultPass,
					Domain: "example.org",
				},
				&authres.DKIMResult{
					Value:  authres.ResultTempError,
					Domain: "example.org",
				},
				&authres.SPFResult{
					Value: authres.ResultNone,
					From:  "example.org",
					Helo:  "mx.example.org",
				},
			},
			output: authres.ResultPass,
		},
		{ // 22
			orgDomain: "example.org",
			record:    &dmarc.Record{},
			results: []authres.Result{
				&authres.DKIMResult{
					Value:  authres.ResultFail,
					Domain: "example.org",
				},
				&authres.DKIMResult{
					Value:  authres.ResultTempError,
					Domain: "example.org",
				},
				&authres.SPFResult{
					Value: authres.ResultNone,
					From:  "example.org",
					Helo:  "mx.example.org",
				},
			},
			output: authres.ResultTempError,
		},
		{ // 23
			orgDomain: "example.org",
			record:    &dmarc.Record{},
			results: []authres.Result{
				&authres.DKIMResult{
					Value:  authres.ResultNone,
					Domain: "example.org",
				},
				&authres.SPFResult{
					Value: authres.ResultNone,
					From:  "example.org",
					Helo:  "mx.example.org",
				},
			},
			output: authres.ResultFail,
		},
	}
	for i, case_ := range cases {
		test(i, case_)
	}
}

func TestExtractDomains(t *testing.T) {
	type tCase struct {
		hdr string

		orgDomain  string
		fromDomain string
	}
	test := func(i int, c tCase) {
		hdr, err := textproto.ReadHeader(bufio.NewReader(strings.NewReader(c.hdr + "\n\n")))
		if err != nil {
			panic(err)
		}

		orgDomain, fromDomain, err := ExtractDomains(hdr)
		if c.orgDomain == "" && err == nil {
			t.Errorf("%d: expected failure, got orgDomain = %s, fromDomain = %s", i, orgDomain, fromDomain)
			return
		}
		if c.orgDomain != "" && err != nil {
			t.Errorf("%d: unexpected error: %v", i, err)
			return
		}
		if orgDomain != c.orgDomain {
			t.Errorf("%d: want orgDomain = %v but got %s", i, c.orgDomain, orgDomain)
		}
		if fromDomain != c.fromDomain {
			t.Errorf("%d: want fromDomain = %v but got %s", i, c.fromDomain, fromDomain)
		}
	}

	cases := []tCase{
		{
			hdr:        `From: <test@example.org>`,
			orgDomain:  "example.org",
			fromDomain: "example.org",
		},
		{
			hdr:        `From: <test@foo.example.org>`,
			orgDomain:  "example.org",
			fromDomain: "foo.example.org",
		},
		{
			hdr: `From: <test@foo.example.org>, <test@bar.example.org>`,
		},
		{
			hdr: `From: <test@foo.example.org>,
From: <test@bar.example.org>`,
		},
		{
			hdr: `From: <test@>`,
		},
		{
			hdr: `From: `,
		},
		{
			hdr: `From: foo`,
		},
		{
			hdr: `From: <test@org>`,
		},
	}
	for i, case_ := range cases {
		test(i, case_)
	}
}
