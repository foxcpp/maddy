package msgpipeline

import (
	"context"
	"errors"
	"math/rand"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-msgauth/authres"
	"github.com/emersion/go-msgauth/dmarc"
	"github.com/foxcpp/maddy/buffer"
	maddydmarc "github.com/foxcpp/maddy/check/dmarc"
	"github.com/foxcpp/maddy/exterrors"
	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/module"
)

// checkRunner runs groups of checks, collects and merges results.
// It also makes sure that each check gets only one state object created.
type checkRunner struct {
	msgMeta      *module.MsgMetadata
	mailFrom     string
	checkedRcpts []string

	doDMARC     bool
	dmarcPolicy chan struct {
		orgDomain  string
		fromDomain string
		record     *dmarc.Record
		err        error
	}
	dmarcPolicyCancel context.CancelFunc

	log log.Logger

	states map[module.Check]module.CheckState

	mergedRes module.CheckResult
}

func newCheckRunner(msgMeta *module.MsgMetadata, log log.Logger) *checkRunner {
	return &checkRunner{
		msgMeta: msgMeta,
		log:     log,
		dmarcPolicy: make(chan struct {
			orgDomain  string
			fromDomain string
			record     *dmarc.Record
			err        error
		}, 1),
		states: make(map[module.Check]module.CheckState),
	}
}

func (cr *checkRunner) checkStates(checks []module.Check) ([]module.CheckState, error) {
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

		cr.log.Debugf("initializing state for %v (%p)", check.(module.Module).InstanceName(), check)
		state, err := check.CheckStateForMsg(cr.msgMeta)
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
	if cr.mailFrom != "" {
		err := cr.runAndMergeResults(newStates, func(s module.CheckState) module.CheckResult {
			res := s.CheckConnection()
			return res
		})
		if err != nil {
			closeStates()
			return nil, err
		}
		err = cr.runAndMergeResults(newStates, func(s module.CheckState) module.CheckResult {
			res := s.CheckSender(cr.mailFrom)
			return res
		})
		if err != nil {
			closeStates()
			return nil, err
		}
	}

	if len(cr.checkedRcpts) != 0 {
		for _, rcpt := range cr.checkedRcpts {
			err := cr.runAndMergeResults(states, func(s module.CheckState) module.CheckResult {
				res := s.CheckRcpt(rcpt)
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
					cr.mergedRes.Header.Add(field.Key(), field.Value())
				}
				data.headerLock.Unlock()
			}

			if subCheckRes.Quarantine {
				data.setQuarantineErr.Do(func() {
					data.quarantineErr = subCheckRes.Reason
				})
			}
			if subCheckRes.Reject {
				data.setRejectErr.Do(func() {
					data.rejectErr = subCheckRes.Reason
				})
			}

			data.wg.Done()
		}()
	}

	data.wg.Wait()
	if data.rejectErr != nil {
		return data.rejectErr
	}

	if data.quarantineErr != nil {
		cr.log.Error("quarantined", data.quarantineErr, "reason", data.quarantineErr.Error())
		cr.mergedRes.Quarantine = true
	}

	return nil
}

func (cr *checkRunner) checkConnSender(checks []module.Check, mailFrom string) error {
	states, err := cr.checkStates(checks)
	if err != nil {
		return err
	}

	err = cr.runAndMergeResults(states, func(s module.CheckState) module.CheckResult {
		res := s.CheckConnection()
		return res
	})
	if err != nil {
		return err
	}
	err = cr.runAndMergeResults(states, func(s module.CheckState) module.CheckResult {
		res := s.CheckSender(mailFrom)
		return res
	})

	cr.mailFrom = mailFrom

	return err
}

func (cr *checkRunner) checkRcpt(checks []module.Check, rcptTo string) error {
	states, err := cr.checkStates(checks)
	if err != nil {
		return err
	}

	err = cr.runAndMergeResults(states, func(s module.CheckState) module.CheckResult {
		res := s.CheckRcpt(rcptTo)
		return res
	})

	cr.checkedRcpts = append(cr.checkedRcpts, rcptTo)
	return err
}

func (cr *checkRunner) checkBody(checks []module.Check, header textproto.Header, body buffer.Buffer) error {
	states, err := cr.checkStates(checks)
	if err != nil {
		return err
	}

	if cr.doDMARC {
		cr.fetchDMARC(header)
	}

	return cr.runAndMergeResults(states, func(s module.CheckState) module.CheckResult {
		res := s.CheckBody(header, body)
		return res
	})
}

func (cr *checkRunner) fetchDMARC(header textproto.Header) {
	var ctx context.Context
	ctx, cr.dmarcPolicyCancel = context.WithCancel(context.Background())
	go func() {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()
				log.Printf("panic during DMARC fetch: %v\n%s", err, stack)
				cr.dmarcPolicy <- struct {
					orgDomain  string
					fromDomain string
					record     *dmarc.Record
					err        error
				}{
					orgDomain:  "",
					fromDomain: "",
					record:     nil,
					err:        exterrors.WithTemporary(errors.New("Internal server error"), true),
				}
			}
		}()

		orgDomain, fromDomain, record, err := maddydmarc.FetchRecord(ctx, header)
		cr.dmarcPolicy <- struct {
			orgDomain  string
			fromDomain string
			record     *dmarc.Record
			err        error
		}{
			orgDomain:  orgDomain,
			fromDomain: fromDomain,
			record:     record,
			err:        err,
		}
	}()
}

func (cr *checkRunner) applyDMARC() error {
	dmarcData := <-cr.dmarcPolicy
	if dmarcData.err != nil {
		return dmarcData.err
	}
	if dmarcData.record == nil {
		cr.log.DebugMsg("no record", "orgdomain", dmarcData.orgDomain)
		return nil
	}

	dmarcFail := false
	var result authres.DMARCResult

	if dmarcData.err != nil {
		result = authres.DMARCResult{
			Value:  authres.ResultPermError,
			Reason: dmarcData.err.Error(),
			From:   dmarcData.orgDomain,
		}
		if dmarc.IsTempFail(dmarcData.err) {
			result.Value = authres.ResultTempError
		}
		cr.mergedRes.AuthResult = append(cr.mergedRes.AuthResult, &result)
		dmarcFail = true
	} else {
		result = maddydmarc.EvaluateAlignment(dmarcData.orgDomain, dmarcData.record, cr.mergedRes.AuthResult)
		cr.mergedRes.AuthResult = append(cr.mergedRes.AuthResult, &result)
		dmarcFail = result.Value != authres.ResultPass
	}

	if !dmarcFail {
		cr.log.DebugMsg("pass", "p", dmarcData.record.Policy, "orgdomain", dmarcData.orgDomain)
		return nil
	}
	// TODO: Report generation.

	if dmarcData.record.Percent != nil && rand.Int31n(100) > int32(*dmarcData.record.Percent) {
		cr.log.Msg("DMARC not enforced due to pct",
			"pct", *dmarcData.record.Percent, "p", dmarcData.record.Policy,
			"orgdomain", dmarcData.orgDomain, "fromdomain", dmarcData.fromDomain)
		return nil
	}

	policy := dmarcData.record.Policy
	if !strings.EqualFold(dmarcData.orgDomain, dmarcData.fromDomain) && dmarcData.record.SubdomainPolicy != "" {
		policy = dmarcData.record.SubdomainPolicy
	}

	switch policy {
	case dmarc.PolicyReject:
		return &exterrors.SMTPError{
			Code:         550,
			EnhancedCode: exterrors.EnhancedCode{5, 7, 1},
			Message:      "DMARC check failed",
			CheckName:    "dmarc",
			Misc: map[string]interface{}{
				"reason":     result.Reason,
				"fromdomain": dmarcData.fromDomain,
				"orgdomain":  dmarcData.orgDomain,
			},
		}
	case dmarc.PolicyQuarantine:
		cr.msgMeta.Quarantine = true

		// Mimick the message structure for regular checks.
		cr.log.Msg("quarantined", "reason", result.Reason, "check", "dmarc",
			"fromdomain", dmarcData.fromDomain, "orgdomain", dmarcData.orgDomain)
	}
	return nil
}

func (cr *checkRunner) applyResults(hostname string, header *textproto.Header) error {
	if cr.mergedRes.Quarantine {
		cr.msgMeta.Quarantine = true
	}

	if cr.doDMARC {
		if err := cr.applyDMARC(); err != nil {
			return err
		}
	}

	// After results for all checks are checked, authRes will be populated with values
	// we should put into Authentication-Results header.
	if len(cr.mergedRes.AuthResult) != 0 {
		header.Add("Authentication-Results", authres.Format(hostname, cr.mergedRes.AuthResult))
	}

	for field := cr.mergedRes.Header.Fields(); field.Next(); {
		header.Add(field.Key(), field.Value())
	}
	return nil
}

func (cr *checkRunner) close() {
	if cr.dmarcPolicyCancel != nil {
		cr.dmarcPolicyCancel()
	}
	for _, state := range cr.states {
		state.Close()
	}
}
