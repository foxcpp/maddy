package modify

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/internal/address"
	"github.com/foxcpp/maddy/internal/buffer"
	"github.com/foxcpp/maddy/internal/config"
	modconfig "github.com/foxcpp/maddy/internal/config/module"
	"github.com/foxcpp/maddy/internal/log"
	"github.com/foxcpp/maddy/internal/module"
)

type Modifier struct {
	modName    string
	instName   string
	inlineArgs []string

	table module.Table

	log log.Logger
}

func New(modName, instName string, _, inlineArgs []string) (module.Module, error) {
	if len(inlineArgs) < 1 {
		return nil, errors.New("specify the table to use")
	}

	return &Modifier{
		modName:    modName,
		instName:   instName,
		inlineArgs: inlineArgs,
		log:        log.Logger{Name: modName},
	}, nil
}

func (m *Modifier) Name() string {
	return m.modName
}

func (m *Modifier) InstanceName() string {
	return m.instName
}

func (m *Modifier) Init(cfg *config.Map) error {
	return modconfig.ModuleFromNode(m.inlineArgs, cfg.Block, cfg.Globals, &m.table)
}

type state struct {
	m *Modifier
}

func (m *Modifier) ModStateForMsg(ctx context.Context, msgMeta *module.MsgMetadata) (module.ModifierState, error) {
	return state{m: m}, nil
}

func (state) RewriteSender(ctx context.Context, from string) (string, error) {
	return from, nil
}

func (s state) RewriteRcpt(ctx context.Context, rcptTo string) (string, error) {
	normAddr, err := address.ForLookup(rcptTo)
	if err != nil {
		return rcptTo, fmt.Errorf("malformed address: %v", err)
	}

	replacement, ok, err := s.m.table.Lookup(normAddr)
	if err == nil && ok {
		if !address.Valid(replacement) {
			return "", fmt.Errorf("refusing to replace recipient with invalid address %s", replacement)
		}
		return replacement, nil
	}

	// Note: be careful to preserve original address case.

	// Okay, then attempt to do rewriting using
	// only mailbox.
	mbox, domain, err := address.Split(normAddr)
	if err != nil {
		// If we have malformed address here, something is really wrong, but let's
		// ignore it silently then anyway.
		return rcptTo, nil
	}

	// mbox is already normalized, since it is a part of address.ForLookup
	// result.
	replacement, ok, err = s.m.table.Lookup(mbox)
	if err == nil && ok {
		if strings.Contains(replacement, "@") && !strings.HasPrefix(replacement, `"`) && !strings.HasSuffix(replacement, `"`) {
			if !address.Valid(replacement) {
				return "", fmt.Errorf("refusing to replace recipient with invalid address %s", replacement)
			}
			return replacement, nil
		}
		return replacement + "@" + domain, nil
	}

	return rcptTo, nil
}

func (state) RewriteBody(ctx context.Context, hdr *textproto.Header, body buffer.Buffer) error {
	return nil
}

func (state) Close() error {
	return nil
}

func init() {
	module.Register("alias", New)
}
