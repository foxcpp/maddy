package modify

import (
	"context"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/internal/buffer"
	"github.com/foxcpp/maddy/internal/module"
)

type (
	// Group wraps multiple modifiers and runs them serially.
	Group struct {
		Modifiers []module.Modifier
	}

	groupState struct {
		states []module.ModifierState
	}
)

func (g Group) ModStateForMsg(ctx context.Context, msgMeta *module.MsgMetadata) (module.ModifierState, error) {
	gs := groupState{}
	for _, modifier := range g.Modifiers {
		state, err := modifier.ModStateForMsg(ctx, msgMeta)
		if err != nil {
			// Free state objects we initialized already.
			for _, state := range gs.states {
				state.Close()
			}
			return nil, err
		}
		gs.states = append(gs.states, state)
	}
	return gs, nil
}

func (gs groupState) RewriteSender(ctx context.Context, mailFrom string) (string, error) {
	var err error
	for _, state := range gs.states {
		mailFrom, err = state.RewriteSender(ctx, mailFrom)
		if err != nil {
			return "", err
		}
	}
	return mailFrom, nil
}

func (gs groupState) RewriteRcpt(ctx context.Context, rcptTo string) (string, error) {
	var err error
	for _, state := range gs.states {
		rcptTo, err = state.RewriteRcpt(ctx, rcptTo)
		if err != nil {
			return "", err
		}
	}
	return rcptTo, nil
}

func (gs groupState) RewriteBody(ctx context.Context, h *textproto.Header, body buffer.Buffer) error {
	for _, state := range gs.states {
		if err := state.RewriteBody(ctx, h, body); err != nil {
			return err
		}
	}
	return nil
}

func (gs groupState) Close() error {
	// We still try close all state objects to minimize
	// resource leaks when Close fails for one object..

	var lastErr error
	for _, state := range gs.states {
		if err := state.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
