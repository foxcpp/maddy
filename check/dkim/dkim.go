package dkim

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-msgauth/authres"
	"github.com/emersion/go-msgauth/dkim"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/check"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/module"
	"github.com/foxcpp/maddy/target"
)

type Check struct {
	instName string
	log      log.Logger

	brokenSigAction check.FailAction
	noSigAction     check.FailAction
}

func New(_, instName string, _, inlineArgs []string) (module.Module, error) {
	if len(inlineArgs) != 0 {
		return nil, errors.New("verify_dkim: inline arguments are not used")
	}
	return &Check{
		instName: instName,
		log:      log.Logger{Name: "verify_dkim"},
	}, nil
}

func (c *Check) Init(cfg *config.Map) error {
	cfg.Bool("debug", true, false, &c.log.Debug)
	cfg.Custom("broken_sig_action", false, false,
		func() (interface{}, error) {
			return check.FailAction{}, nil
		}, check.FailActionDirective, &c.brokenSigAction)
	cfg.Custom("no_sig_action", false, false,
		func() (interface{}, error) {
			return check.FailAction{}, nil
		}, check.FailActionDirective, &c.noSigAction)
	_, err := cfg.Process()
	if err != nil {
		return err
	}
	return nil
}

func (c *Check) Name() string {
	return "verify_dkim"
}

func (c *Check) InstanceName() string {
	return c.instName
}

type dkimCheckState struct {
	c       *Check
	msgMeta *module.MsgMetadata
	log     log.Logger
}

func (d dkimCheckState) CheckConnection(ctx context.Context) module.CheckResult {
	return module.CheckResult{}
}

func (d dkimCheckState) CheckSender(ctx context.Context, mailFrom string) module.CheckResult {
	return module.CheckResult{}
}

func (d dkimCheckState) CheckRcpt(ctx context.Context, rcptTo string) module.CheckResult {
	return module.CheckResult{}
}

func (d dkimCheckState) CheckBody(ctx context.Context, header textproto.Header, body buffer.Buffer) module.CheckResult {
	if !header.Has("DKIM-Signature") {
		return d.c.noSigAction.Apply(module.CheckResult{
			RejectErr: &smtp.SMTPError{
				Code:         550,
				EnhancedCode: smtp.EnhancedCode{5, 7, 20},
				Message:      "No DKIM signatures present",
			},
			AuthResult: []authres.Result{
				&authres.DKIMResult{
					Value: authres.ResultNone,
				},
			},
		})
	}

	// TODO: Optimize that so we can avoid serializing header once more.
	// https://github.com/emersion/go-msgauth/issues/10.
	b := bytes.Buffer{}
	textproto.WriteHeader(&b, header)
	bodyRdr, err := body.Open()
	if err != nil {
		return module.CheckResult{RejectErr: err}
	}

	verifications, err := dkim.Verify(io.MultiReader(&b, bodyRdr))
	if err != nil {
		return module.CheckResult{RejectErr: err}
	}

	brokenSigs := false

	res := module.CheckResult{AuthResult: make([]authres.Result, 0, len(verifications))}
	for _, verif := range verifications {
		var val authres.ResultValue
		if verif.Err == nil {
			val = authres.ResultPass
		} else {
			if dkim.IsPermFail(err) {
				brokenSigs = true
			}
			val = authres.ResultFail
		}

		res.AuthResult = append(res.AuthResult, &authres.DKIMResult{
			Value:      val,
			Domain:     verif.Domain,
			Identifier: verif.Identifier,
		})
	}

	if brokenSigs {
		d.log.Println("broken signatures present")
		return d.c.brokenSigAction.Apply(res)
	}
	return res
}

func (d dkimCheckState) Close() error {
	return nil
}

func (c *Check) CheckStateForMsg(msgMeta *module.MsgMetadata) (module.CheckState, error) {
	return dkimCheckState{
		c:       c,
		msgMeta: msgMeta,
		log:     target.DeliveryLogger(c.log, msgMeta),
	}, nil
}

func init() {
	module.Register("verify_dkim", New)
	module.RegisterInstance(&Check{
		instName: "verify_dkim",
		log:      log.Logger{Name: "verify_dkim"},
	}, &config.Map{Block: &config.Node{}})
}
