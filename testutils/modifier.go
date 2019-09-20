package testutils

import (
	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/module"
)

type Modifier struct {
	InstName string

	InitErr     error
	MailFromErr error
	RcptToErr   error
	BodyErr     error

	MailFrom map[string]string
	RcptTo   map[string]string
	AddHdr   textproto.Header

	UnclosedStates int
}

func (m Modifier) Init(*config.Map) error {
	return nil
}

func (m Modifier) Name() string {
	return "test_modifier"
}

func (m Modifier) InstanceName() string {
	return m.InstName
}

type modifierState struct {
	m *Modifier
}

func (m Modifier) ModStateForMsg(msgMeta *module.MsgMetadata) (module.ModifierState, error) {
	if m.InitErr != nil {
		return nil, m.InitErr
	}

	m.UnclosedStates++
	return modifierState{&m}, nil
}

func (ms modifierState) RewriteSender(mailFrom string) (string, error) {
	if ms.m.MailFromErr != nil {
		return "", ms.m.MailFromErr
	}
	if ms.m.MailFrom == nil {
		return mailFrom, nil
	}

	newMailFrom, ok := ms.m.MailFrom[mailFrom]
	if ok {
		return newMailFrom, nil
	}
	return mailFrom, nil
}

func (ms modifierState) RewriteRcpt(rcptTo string) (string, error) {
	if ms.m.RcptToErr != nil {
		return "", ms.m.RcptToErr
	}

	if ms.m.RcptTo == nil {
		return rcptTo, nil
	}

	newRcptTo, ok := ms.m.RcptTo[rcptTo]
	if ok {
		return newRcptTo, nil
	}
	return rcptTo, nil
}

func (ms modifierState) RewriteBody(h textproto.Header, body buffer.Buffer) error {
	if ms.m.BodyErr != nil {
		return ms.m.BodyErr
	}

	for field := ms.m.AddHdr.Fields(); field.Next(); {
		h.Add(field.Key(), field.Value())
	}
	return nil
}

func (ms modifierState) Close() error {
	ms.m.UnclosedStates--
	return nil
}

func init() {
	module.Register("test_modifier", func(_, _ string, _, _ []string) (module.Module, error) {
		return &Modifier{}, nil
	})
	module.RegisterInstance(&Modifier{}, nil)
}
