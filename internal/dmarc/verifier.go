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
	"context"
	"math/rand"
	"net"
	"runtime/trace"
	"strings"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-msgauth/authres"
	"github.com/emersion/go-msgauth/dmarc"
)

type verifyData struct {
	policyDomain string
	fromDomain   string
	record       *Record
	recordErr    error
}

// errPanic is used to propagate the panic() from the FetchRecord
// goroutine to the goroutine that called Apply.
type errPanic struct {
	err interface{}
}

func (errPanic) Error() string {
	return "panic during policy fetch"
}

// Verifier is the structure that wraps all state necessary to verify a
// single message using DMARC checks.
//
// It cannot be reused.
type Verifier struct {
	fetchCh     chan verifyData
	fetchCancel context.CancelFunc

	resolver Resolver

	// TODO(GH #206): DMARC reporting
	// FailureReportFunc is the callback that is called when a failure report
	// is generated. If it is nil - failure reports generation is disabled.
	// FailureReportFunc func(textproto.Header, io.Reader)
}

func NewVerifier(r Resolver) *Verifier {
	return &Verifier{
		fetchCh:  make(chan verifyData, 1),
		resolver: r,
	}
}

func (v *Verifier) Close() error {
	if v.fetchCancel != nil {
		v.fetchCancel()
	}
	return nil
}

// FetchRecord prepares the Verifier by starting the policy lookup. Lookup is
// performed asynchronously to improve performance.
//
// If panic occurs in the lookup goroutine - call to Apply will panic.
func (v *Verifier) FetchRecord(ctx context.Context, header textproto.Header) {
	fromDomain, err := ExtractFromDomain(header)
	if err != nil {
		v.fetchCh <- verifyData{
			recordErr: err,
		}
		return
	}

	ctx, v.fetchCancel = context.WithCancel(ctx)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				v.fetchCh <- verifyData{
					recordErr: errPanic{err: err},
				}
			}
		}()

		defer trace.StartRegion(ctx, "DMARC/FetchRecord").End()

		policyDomain, record, err := FetchRecord(ctx, v.resolver, fromDomain)
		v.fetchCh <- verifyData{
			policyDomain: policyDomain,
			fromDomain:   fromDomain,
			record:       record,
			recordErr:    err,
		}
	}()
}

// Apply actually performs all actions necessary to apply a DMARC policy to the message.
//
// The authRes slice should contain results for DKIM and SPF checks. FetchRecord should be
// caled before calling this function.
//
// It returns the Authentication-Result field to be included in the message (as
// a part of the EvalResult struct) and the appropriate action that should be
// taken by the MTA. In case of PolicyReject, caller should inspect the
// Result.Value to determine whether to use a temporary or permanent error code
// as Apply implements the 'fail closed' strategy for handling of temporary
// errors.
//
// Additionally, it relies on the math/rand default source to be initialized to determine
// whether to apply a policy with the pct key.
func (v *Verifier) Apply(authRes []authres.Result) (EvalResult, Policy) {
	data := <-v.fetchCh
	if data.recordErr != nil {
		result := authres.DMARCResult{
			Value:  authres.ResultPermError,
			Reason: "Policy lookup failed: " + data.recordErr.Error(),
			// If may be empty, but it is fine (it will not be included in the field then).
			From: data.fromDomain,
		}
		if dnsErr, ok := data.recordErr.(*net.DNSError); ok && dnsErr.Temporary() {
			result.Value = authres.ResultTempError
			// 'fail closed' behavior, reject the message if a temporary error
			// occurs.
			return EvalResult{
				Authres: result,
			}, dmarc.PolicyReject
		}
		return EvalResult{
			Authres: result,
		}, dmarc.PolicyNone
	}
	if data.record == nil {
		return EvalResult{
			Authres: authres.DMARCResult{
				Value: authres.ResultNone,
				From:  data.fromDomain,
			},
		}, dmarc.PolicyNone
	}

	result := EvaluateAlignment(data.fromDomain, data.record, authRes)
	if result.Authres.Value == authres.ResultPass || result.Authres.Value == authres.ResultNone {
		return result, dmarc.PolicyNone
	}

	if data.record.Percent != nil && rand.Int31n(100) > int32(*data.record.Percent) {
		return result, dmarc.PolicyNone
	}

	policy := data.record.Policy
	if !strings.EqualFold(data.policyDomain, data.fromDomain) && data.record.SubdomainPolicy != "" {
		policy = data.record.SubdomainPolicy
	}

	return result, policy
}
