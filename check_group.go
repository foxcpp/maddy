package maddy

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-msgauth/authres"
	"github.com/foxcpp/maddy/atomicbool"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/module"
	"golang.org/x/sync/errgroup"
)

// CheckGroup is a type that wraps a group of checks and runs them in parallel.
//
// It implements module.Check interface.
type CheckGroup struct {
	checks []module.Check
}

func (cg *CheckGroup) NewMessage(msgMeta *module.MsgMetadata) (module.CheckState, error) {
	states := make([]module.CheckState, 0, len(cg.checks))
	for _, check := range cg.checks {
		state, err := check.NewMessage(msgMeta)
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return &checkGroupState{msgMeta, states}, nil
}

type checkGroupState struct {
	msgMeta *module.MsgMetadata
	states  []module.CheckState
}

func (cgs *checkGroupState) runAndMergeResults(ctx context.Context, runner func(context.Context, module.CheckState) module.CheckResult) module.CheckResult {
	var (
		checkScore     int32
		quarantineFlag atomicbool.AtomicBool
		authRes        []authres.Result
		authResLock    sync.Mutex
		header         textproto.Header
		headerLock     sync.Mutex
	)

	syncGroup, childCtx := errgroup.WithContext(ctx)
	for _, state := range cgs.states {
		state := state
		syncGroup.Go(func() error {
			subCheckRes := runner(childCtx, state)

			// We check the length because we don't want to take locks
			// when it is not necessary.
			if len(subCheckRes.AuthResult) != 0 {
				authResLock.Lock()
				authRes = append(authRes, subCheckRes.AuthResult...)
				authResLock.Unlock()
			}
			if subCheckRes.Header.Len() != 0 {
				headerLock.Lock()
				for field := subCheckRes.Header.Fields(); field.Next(); {
					header.Add(field.Key(), field.Value())
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
	return module.CheckResult{
		RejectErr:   err,
		Quarantine:  quarantineFlag.IsSet(),
		ScoreAdjust: checkScore,
		AuthResult:  authRes,
		Header:      header,
	}
}

func (cgs *checkGroupState) CheckConnection(ctx context.Context) module.CheckResult {
	return cgs.runAndMergeResults(ctx, func(childCtx context.Context, state module.CheckState) module.CheckResult {
		return state.CheckConnection(childCtx)
	})
}

func (cgs *checkGroupState) CheckSender(ctx context.Context, from string) module.CheckResult {
	return cgs.runAndMergeResults(ctx, func(childCtx context.Context, state module.CheckState) module.CheckResult {
		return state.CheckSender(childCtx, from)
	})
}

func (cgs *checkGroupState) CheckRcpt(ctx context.Context, to string) module.CheckResult {
	return cgs.runAndMergeResults(ctx, func(childCtx context.Context, state module.CheckState) module.CheckResult {
		return state.CheckRcpt(childCtx, to)
	})
}

func (cgs *checkGroupState) CheckBody(ctx context.Context, header textproto.Header, body buffer.Buffer) module.CheckResult {
	return cgs.runAndMergeResults(ctx, func(childCtx context.Context, state module.CheckState) module.CheckResult {
		return state.CheckBody(childCtx, header, body)
	})
}

func (cgs *checkGroupState) Close() error {
	for _, state := range cgs.states {
		state.Close()
	}
	return nil
}
