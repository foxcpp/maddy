package dispatcher

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-msgauth/authres"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/atomicbool"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/module"
)

// checkRunner runs groups of checks, collects and merges results.
// It takes care of quarantine/reject scores.
// It also makes sure that each check gets only one state object created.
type checkRunner struct {
	msgMeta      *module.MsgMetadata
	mailFrom     string
	checkedRcpts []string

	quarantineScore *int
	rejectScore     *int

	log log.Logger

	states map[module.Check]module.CheckState

	mergedRes module.CheckResult
}

func newCheckRunner(msgMeta *module.MsgMetadata, log log.Logger, quarantineScore, rejectScore *int) *checkRunner {
	return &checkRunner{
		msgMeta:         msgMeta,
		quarantineScore: quarantineScore,
		rejectScore:     rejectScore,
		log:             log,
		states:          make(map[module.Check]module.CheckState),
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
		checkScore     int32
		quarantineFlag atomicbool.AtomicBool
		authResLock    sync.Mutex
		headerLock     sync.Mutex
		firstErr       error

		setErr sync.Once
		wg     sync.WaitGroup
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

			if subCheckRes.ScoreAdjust != 0 {
				atomic.AddInt32(&data.checkScore, subCheckRes.ScoreAdjust)
			}
			if subCheckRes.Quarantine {
				data.quarantineFlag.Set(true)
			}
			if subCheckRes.RejectErr != nil {
				data.setErr.Do(func() { data.firstErr = subCheckRes.RejectErr })
			}

			data.wg.Done()
		}()
	}

	data.wg.Wait()
	if data.firstErr != nil {
		return data.firstErr
	}

	if data.quarantineFlag.IsSet() {
		cr.mergedRes.Quarantine = true
	}
	cr.mergedRes.ScoreAdjust += data.checkScore

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

	return cr.runAndMergeResults(states, func(s module.CheckState) module.CheckResult {
		res := s.CheckBody(header, body)
		return res
	})
}

func (cr *checkRunner) applyResults(hostname string, header *textproto.Header) error {
	if cr.mergedRes.Quarantine {
		cr.log.Printf("quarantined message due to check result")
		cr.msgMeta.Quarantine = true
	}

	checkScore := cr.mergedRes.ScoreAdjust

	if cr.rejectScore != nil && checkScore >= int32(*cr.rejectScore) {
		cr.log.Debugf("score %d >= %d, rejecting", checkScore, *cr.rejectScore)
		return &smtp.SMTPError{
			Code:         550,
			EnhancedCode: smtp.EnhancedCode{5, 7, 0},
			Message:      fmt.Sprintf("Message is rejected due to multiple local policy violations (score %d)", checkScore),
		}
	}
	if cr.quarantineScore != nil && checkScore >= int32(*cr.quarantineScore) {
		if !cr.msgMeta.Quarantine {
			cr.log.Printf("quarantined message due to score %d >= %d", checkScore, *cr.quarantineScore)
		}
		cr.msgMeta.Quarantine = true
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
	for _, state := range cr.states {
		state.Close()
	}
}
