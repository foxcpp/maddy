package dispatcher

import (
	"context"
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
	"golang.org/x/sync/errgroup"
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

		cr.log.Debugf("initializing state for %T/%v (%p)", check, check.(module.Module).InstanceName(), check)
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
		err := cr.runAndMergeResults(ctx, newStates, func(ctx context.Context, s module.CheckState) module.CheckResult {
			res := s.CheckConnection(ctx)
			cr.log.Debugf("(replay for new state) %T.CheckConnection: %+v", s, res)
			return res
		})
		if err != nil {
			closeStates()
			return nil, err
		}
		err = cr.runAndMergeResults(ctx, newStates, func(ctx context.Context, s module.CheckState) module.CheckResult {
			res := s.CheckSender(ctx, cr.mailFrom)
			cr.log.Debugf("(replay for new state) %T.CheckSender %s: %+v", s, cr.mailFrom, res)
			return res
		})
		if err != nil {
			closeStates()
			return nil, err
		}
	}

	if len(cr.checkedRcpts) != 0 {
		for _, rcpt := range cr.checkedRcpts {
			err := cr.runAndMergeResults(ctx, states, func(ctx context.Context, s module.CheckState) module.CheckResult {
				res := s.CheckRcpt(ctx, rcpt)
				cr.log.Debugf("(replay for new state) %T.CheckRcpt %s: %+v", s, rcpt, res)
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
	// TODO: Check whether all this bookkeeping leads to high GC pressure.
	for check, state := range newStatesMap {
		cr.states[check] = state
	}

	return states, nil
}

func (cr *checkRunner) runAndMergeResults(ctx context.Context, states []module.CheckState, runner func(context.Context, module.CheckState) module.CheckResult) error {
	var (
		checkScore     int32
		quarantineFlag atomicbool.AtomicBool
		authResLock    sync.Mutex
		headerLock     sync.Mutex
	)

	syncGroup, childCtx := errgroup.WithContext(ctx)
	for _, state := range states {
		state := state
		syncGroup.Go(func() error {
			subCheckRes := runner(childCtx, state)

			// We check the length because we don't want to take locks
			// when it is not necessary.
			if len(subCheckRes.AuthResult) != 0 {
				authResLock.Lock()
				cr.mergedRes.AuthResult = append(cr.mergedRes.AuthResult, subCheckRes.AuthResult...)
				authResLock.Unlock()
			}
			if subCheckRes.Header.Len() != 0 {
				headerLock.Lock()
				for field := subCheckRes.Header.Fields(); field.Next(); {
					cr.mergedRes.Header.Add(field.Key(), field.Value())
				}
				headerLock.Unlock()
			}

			if subCheckRes.ScoreAdjust != 0 {
				atomic.AddInt32(&checkScore, subCheckRes.ScoreAdjust)
			}
			if subCheckRes.Quarantine {
				quarantineFlag.Set(true)
			}
			if subCheckRes.RejectErr != nil {
				return subCheckRes.RejectErr
			}
			return nil
		})
	}

	err := syncGroup.Wait()
	if err != nil {
		return err
	}

	if quarantineFlag.IsSet() {
		cr.mergedRes.Quarantine = true
	}
	cr.mergedRes.ScoreAdjust += checkScore

	return nil
}

func (cr *checkRunner) checkConnSender(ctx context.Context, checks []module.Check, mailFrom string) error {
	states, err := cr.checkStates(ctx, checks)
	if err != nil {
		return err
	}

	err = cr.runAndMergeResults(ctx, states, func(ctx context.Context, s module.CheckState) module.CheckResult {
		res := s.CheckConnection(ctx)
		cr.log.Debugf("%T.CheckConnection: %+v", s, res)
		return res
	})
	if err != nil {
		return err
	}
	err = cr.runAndMergeResults(ctx, states, func(ctx context.Context, s module.CheckState) module.CheckResult {
		res := s.CheckSender(ctx, mailFrom)
		cr.log.Debugf("%T.CheckSender %s: %+v", s, mailFrom, res)
		return res
	})

	cr.mailFrom = mailFrom

	return err
}

func (cr *checkRunner) checkRcpt(ctx context.Context, checks []module.Check, rcptTo string) error {
	states, err := cr.checkStates(ctx, checks)
	if err != nil {
		return err
	}

	err = cr.runAndMergeResults(ctx, states, func(ctx context.Context, s module.CheckState) module.CheckResult {
		res := s.CheckRcpt(ctx, rcptTo)
		cr.log.Debugf("%T.CheckRcpt %s: %+v", s, rcptTo, res)
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

	return cr.runAndMergeResults(ctx, states, func(ctx context.Context, s module.CheckState) module.CheckResult {
		res := s.CheckBody(ctx, header, body)
		cr.log.Debugf("%T.CheckBody: %+v", s, res)
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
