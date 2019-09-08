package check

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

// Group is a type that wraps a group of checks and runs them in parallel.
//
// It implements module.Check interface.
type Group struct {
	Checks []module.Check
}

func (g *Group) NewMessage(msgMeta *module.MsgMetadata) (module.CheckState, error) {
	states := make([]module.CheckState, 0, len(g.Checks))
	for _, check := range g.Checks {
		state, err := check.NewMessage(msgMeta)
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return &groupState{msgMeta, states}, nil
}

type groupState struct {
	msgMeta *module.MsgMetadata
	states  []module.CheckState
}

func (gs *groupState) runAndMergeResults(ctx context.Context, runner func(context.Context, module.CheckState) module.CheckResult) module.CheckResult {
	var (
		checkScore     int32
		quarantineFlag atomicbool.AtomicBool
		authRes        []authres.Result
		authResLock    sync.Mutex
		header         textproto.Header
		headerLock     sync.Mutex
	)

	syncGroup, childCtx := errgroup.WithContext(ctx)
	for _, state := range gs.states {
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

func (gs *groupState) CheckConnection(ctx context.Context) module.CheckResult {
	return gs.runAndMergeResults(ctx, func(childCtx context.Context, state module.CheckState) module.CheckResult {
		return state.CheckConnection(childCtx)
	})
}

func (gs *groupState) CheckSender(ctx context.Context, from string) module.CheckResult {
	return gs.runAndMergeResults(ctx, func(childCtx context.Context, state module.CheckState) module.CheckResult {
		return state.CheckSender(childCtx, from)
	})
}

func (gs *groupState) CheckRcpt(ctx context.Context, to string) module.CheckResult {
	return gs.runAndMergeResults(ctx, func(childCtx context.Context, state module.CheckState) module.CheckResult {
		return state.CheckRcpt(childCtx, to)
	})
}

func (gs *groupState) CheckBody(ctx context.Context, header textproto.Header, body buffer.Buffer) module.CheckResult {
	return gs.runAndMergeResults(ctx, func(childCtx context.Context, state module.CheckState) module.CheckResult {
		return state.CheckBody(childCtx, header, body)
	})
}

func (gs *groupState) Close() error {
	for _, state := range gs.states {
		state.Close()
	}
	return nil
}
