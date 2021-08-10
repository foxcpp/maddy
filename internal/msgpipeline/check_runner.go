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
	"runtime/debug"
	"sync"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-msgauth/authres"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/dns"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/dmarc"
)

// checkRunner runs groups of checks, collects and merges results.
// It also makes sure that each check gets only one state object created.
type checkRunner struct {
	msgMeta          *module.MsgMetadata
	mailFrom         string
	mailFromReceived bool

	checkedRcpts         []string
	checkedRcptsPerCheck map[module.CheckState]map[string]struct{}
	checkedRcptsLock     sync.Mutex

	resolver      dns.Resolver
	doDMARC       bool
	didDMARCFetch bool
	dmarcVerify   *dmarc.Verifier

	log log.Logger

	states map[module.Check]module.CheckState

	mergedRes module.CheckResult
}

func newCheckRunner(msgMeta *module.MsgMetadata, log log.Logger, r dns.Resolver) *checkRunner {
	return &checkRunner{
		msgMeta:              msgMeta,
		checkedRcptsPerCheck: map[module.CheckState]map[string]struct{}{},
		log:                  log,
		resolver:             r,
		dmarcVerify:          dmarc.NewVerifier(r),
		states:               make(map[module.Check]module.CheckState),
	}
}

func (cr *checkRunner) checkStates(ctx context.Context, checks []module.Check) ([]module.CheckState, error) {
	states := make([]module.CheckState, 0, len(checks))
	newStates := make([]module.CheckState, 0, len(checks))
	newStatesMap := make(map[module.Check]module.CheckState, len(checks))
	closeStates := func() {
		for _, state := range states {
			state.Close()
		}
	}

	for _, check := range checks {
		state, ok := cr.states[check]
		if ok {
			states = append(states, state)
			continue
		}

		cr.log.Debugf("initializing state for %v (%p)", objectName(check), check)
		state, err := check.CheckStateForMsg(ctx, cr.msgMeta)
		if err != nil {
			closeStates()
			return nil, err
		}
		states = append(states, state)
		newStates = append(newStates, state)
		newStatesMap[check] = state
	}

	if len(newStates) == 0 {
		return states, nil
	}

	// Here we replay previous CheckConnection/CheckSender/CheckRcpt calls
	// for any newly initialized checks so they all get change to see all these things.
	//
	// Done outside of check loop above to make sure we can run these for multiple
	// checks in parallel.
	if cr.mailFromReceived {
		err := cr.runAndMergeResults(newStates, func(s module.CheckState) module.CheckResult {
			res := s.CheckConnection(ctx)
			return res
		})
		if err != nil {
			closeStates()
			return nil, err
		}
		err = cr.runAndMergeResults(newStates, func(s module.CheckState) module.CheckResult {
			res := s.CheckSender(ctx, cr.mailFrom)
			return res
		})
		if err != nil {
			closeStates()
			return nil, err
		}
	}

	if len(cr.checkedRcpts) != 0 {
		for _, rcpt := range cr.checkedRcpts {
			rcpt := rcpt
			err := cr.runAndMergeResults(states, func(s module.CheckState) module.CheckResult {
				// Avoid calling CheckRcpt for the same recipient for the same check
				// multiple times, even if requested.
				cr.checkedRcptsLock.Lock()
				if _, ok := cr.checkedRcptsPerCheck[s][rcpt]; ok {
					cr.checkedRcptsLock.Unlock()
					return module.CheckResult{}
				}
				if cr.checkedRcptsPerCheck[s] == nil {
					cr.checkedRcptsPerCheck[s] = make(map[string]struct{})
				}
				cr.checkedRcptsPerCheck[s][rcpt] = struct{}{}
				cr.checkedRcptsLock.Unlock()

				res := s.CheckRcpt(ctx, rcpt)
				return res
			})
			if err != nil {
				closeStates()
				return nil, err
			}
		}
	}

	// This is done after all actions that can fail so we will not have to remove
	// state objects from main map.
	for check, state := range newStatesMap {
		cr.states[check] = state
	}

	return states, nil
}

func (cr *checkRunner) runAndMergeResults(states []module.CheckState, runner func(module.CheckState) module.CheckResult) error {
	data := struct {
		authResLock sync.Mutex
		headerLock  sync.Mutex

		quarantineErr    error
		quarantineCheck  string
		setQuarantineErr sync.Once

		rejectErr    error
		rejectCheck  string
		setRejectErr sync.Once

		wg sync.WaitGroup
	}{}

	for _, state := range states {
		state := state
		data.wg.Add(1)
		go func() {
			defer func() {
				data.wg.Done()
				if err := recover(); err != nil {
					stack := debug.Stack()
					log.Printf("panic during check execution: %v\n%s", err, stack)
				}
			}()

			subCheckRes := runner(state)

			// We check the length because we don't want to take locks
			// when it is not necessary.
			if len(subCheckRes.AuthResult) != 0 {
				data.authResLock.Lock()
				cr.mergedRes.AuthResult = append(cr.mergedRes.AuthResult, subCheckRes.AuthResult...)
				data.authResLock.Unlock()
			}
			if subCheckRes.Header.Len() != 0 {
				data.headerLock.Lock()
				for field := subCheckRes.Header.Fields(); field.Next(); {
					formatted, err := field.Raw()
					if err != nil {
						cr.log.Error("malformed header field added by check", err)
					}
					cr.mergedRes.Header.AddRaw(formatted)
				}
				data.headerLock.Unlock()
			}

			if subCheckRes.Quarantine {
				data.setQuarantineErr.Do(func() {
					data.quarantineErr = subCheckRes.Reason
				})
			} else if subCheckRes.Reject {
				data.setRejectErr.Do(func() {
					data.rejectErr = subCheckRes.Reason
				})
			} else if subCheckRes.Reason != nil {
				// 'action ignore' case. There is Reason, but action.Apply set
				// both Reject and Quarantine to false. Log the reason for
				// purposes of deployment testing.
				cr.log.Error("no check action", subCheckRes.Reason)
			}
		}()
	}

	data.wg.Wait()
	if data.rejectErr != nil {
		return data.rejectErr
	}

	if data.quarantineErr != nil {
		cr.log.Error("quarantined", data.quarantineErr)
		cr.mergedRes.Quarantine = true
	}

	return nil
}

func (cr *checkRunner) checkConnSender(ctx context.Context, checks []module.Check, mailFrom string) error {
	cr.mailFrom = mailFrom
	cr.mailFromReceived = true

	// checkStates will run CheckConnection and CheckSender.
	_, err := cr.checkStates(ctx, checks)
	return err
}

func (cr *checkRunner) checkRcpt(ctx context.Context, checks []module.Check, rcptTo string) error {
	states, err := cr.checkStates(ctx, checks)
	if err != nil {
		return err
	}

	err = cr.runAndMergeResults(states, func(s module.CheckState) module.CheckResult {
		cr.checkedRcptsLock.Lock()
		if _, ok := cr.checkedRcptsPerCheck[s][rcptTo]; ok {
			cr.checkedRcptsLock.Unlock()
			return module.CheckResult{}
		}
		if cr.checkedRcptsPerCheck[s] == nil {
			cr.checkedRcptsPerCheck[s] = make(map[string]struct{})
		}
		cr.checkedRcptsPerCheck[s][rcptTo] = struct{}{}
		cr.checkedRcptsLock.Unlock()

		res := s.CheckRcpt(ctx, rcptTo)
		return res
	})

	cr.checkedRcpts = append(cr.checkedRcpts, rcptTo)
	return err
}

func (cr *checkRunner) checkBody(ctx context.Context, checks []module.Check, header textproto.Header, body buffer.Buffer) error {
	states, err := cr.checkStates(ctx, checks)
	if err != nil {
		return err
	}

	if cr.doDMARC && !cr.didDMARCFetch {
		cr.dmarcVerify.FetchRecord(ctx, header)
		cr.didDMARCFetch = true
	}

	return cr.runAndMergeResults(states, func(s module.CheckState) module.CheckResult {
		res := s.CheckBody(ctx, header, body)
		return res
	})
}

func (cr *checkRunner) applyResults(hostname string, header *textproto.Header) error {
	if cr.mergedRes.Quarantine {
		cr.msgMeta.Quarantine = true
	}

	if cr.doDMARC {
		dmarcRes, policy := cr.dmarcVerify.Apply(cr.mergedRes.AuthResult)
		cr.mergedRes.AuthResult = append(cr.mergedRes.AuthResult, &dmarcRes.Authres)
		switch policy {
		case dmarc.PolicyReject:
			code := 550
			enchCode := exterrors.EnhancedCode{5, 7, 1}
			if dmarcRes.Authres.Value == authres.ResultTempError {
				code = 450
				enchCode[0] = 4
			}
			return &exterrors.SMTPError{
				Code:         code,
				EnhancedCode: enchCode,
				Message:      "DMARC check failed",
				CheckName:    "dmarc",
				Misc: map[string]interface{}{
					"reason":      dmarcRes.Authres.Reason,
					"dkim_res":    dmarcRes.DKIMResult.Value,
					"dkim_domain": dmarcRes.DKIMResult.Domain,
					"spf_res":     dmarcRes.SPFResult.Value,
					"spf_from":    dmarcRes.SPFResult.From,
				},
			}
		case dmarc.PolicyQuarantine:
			cr.msgMeta.Quarantine = true

			// Mimick the message structure for regular checks.
			cr.log.Msg("quarantined", "reason", dmarcRes.Authres.Reason, "check", "dmarc")
		}
	}

	// After results for all checks are checked, authRes will be populated with values
	// we should put into Authentication-Results header.
	if len(cr.mergedRes.AuthResult) != 0 {
		header.Add("Authentication-Results", authres.Format(hostname, cr.mergedRes.AuthResult))
	}

	for field := cr.mergedRes.Header.Fields(); field.Next(); {
		formatted, err := field.Raw()
		if err != nil {
			cr.log.Error("malformed header field added by check", err)
		}
		header.AddRaw(formatted)
	}
	return nil
}

func (cr *checkRunner) close() {
	cr.dmarcVerify.Close()
	for _, state := range cr.states {
		state.Close()
	}
}
