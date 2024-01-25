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
		fromDomain string
		record     *Record
		results    []authres.Result

		output authres.ResultValue
	}
	test := func(i int, c tCase) {
		out := EvaluateAlignment(c.fromDomain, c.record, c.results)
		t.Logf("%d - %+v", i, out)
		if out.Authres.Value != c.output {
			t.Errorf("%d: Wrong eval result, want '%s', got '%s' (%+v)", i, c.output, out.Authres.Value, out)
		}
	}

	cases := []tCase{
		{ // 0
			fromDomain: "example.org",
			record:     &Record{},

			output: authres.ResultNone,
		},
		{ // 1
			fromDomain: "example.org",
			record:     &Record{},
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
			fromDomain: "example.org",
			record:     &Record{},
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
			fromDomain: "example.org",
			record:     &Record{},
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
			fromDomain: "example.org",
			record:     &Record{},
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
			fromDomain: "example.com",
			record:     &Record{},
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
			fromDomain: "example.com",
			record: &Record{
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
			fromDomain: "example.org",
			record:     &Record{},
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
			fromDomain: "example.org",
			record:     &Record{},
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
			fromDomain: "example.com",
			record:     &Record{},
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
			fromDomain: "example.com",
			record: &Record{
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
			fromDomain: "example.com",
			record: &Record{
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
			fromDomain: "example.com",
			record: &Record{
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
			fromDomain: "example.org",
			record:     &Record{},
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
			fromDomain: "example.com",
			record:     &Record{},
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
			fromDomain: "example.com",
			record: &Record{
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
			fromDomain: "example.com",
			record:     &Record{},
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
		{ // 17
			fromDomain: "example.com",
			record:     &Record{},
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
		{ // 18
			fromDomain: "example.com",
			record:     &Record{},
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
		{ // 19
			fromDomain: "example.com",
			record:     &Record{},
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
		{ // 20
			fromDomain: "example.org",
			record:     &Record{},
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
		{ // 21
			fromDomain: "example.org",
			record:     &Record{},
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
		{ // 22
			fromDomain: "example.org",
			record:     &Record{},
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
		{ // 23
			fromDomain: "sub.example.org",
			record:     &Record{},
			results: []authres.Result{
				&authres.DKIMResult{
					Value:  authres.ResultPass,
					Domain: "mx.example.org",
				},
				&authres.SPFResult{
					Value: authres.ResultNone,
					From:  "example.org",
					Helo:  "mx.example.org",
				},
			},
			output: authres.ResultPass,
		},
	}
	for i, case_ := range cases {
		test(i, case_)
	}
}

func TestExtractDomains(t *testing.T) {
	type tCase struct {
		hdr string

		fromDomain string
	}
	test := func(i int, c tCase) {
		hdr, err := textproto.ReadHeader(bufio.NewReader(strings.NewReader(c.hdr + "\n\n")))
		if err != nil {
			panic(err)
		}

		domain, err := ExtractFromDomain(hdr)
		if c.fromDomain == "" && err == nil {
			t.Errorf("%d: expected failure, got fromDomain = %s", i, domain)
			return
		}
		if c.fromDomain != "" && err != nil {
			t.Errorf("%d: unexpected error: %v", i, err)
			return
		}
		if domain != c.fromDomain {
			t.Errorf("%d: want fromDomain = %v but got %s", i, c.fromDomain, domain)
		}
	}

	cases := []tCase{
		{
			hdr:        `From: <test@example.org>`,
			fromDomain: "example.org",
		},
		{
			hdr:        `From: <test@foo.example.org>`,
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
	}
	for i, case_ := range cases {
		test(i, case_)
	}
}
