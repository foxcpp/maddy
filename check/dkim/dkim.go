package dkim

import (
	"bytes"
	"errors"
	"io"
	nettextproto "net/textproto"
	"strings"

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

	requiredFields  map[string]struct{}
	allowBodySubset bool
	brokenSigAction check.FailAction
	noSigAction     check.FailAction
	okScore         int32
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
	var requiredFields []string

	cfg.Bool("debug", true, false, &c.log.Debug)
	cfg.StringList("required_fields", false, false, []string{"From", "Subject"}, &requiredFields)
	cfg.Bool("allow_body_subset", false, false, &c.allowBodySubset)
	cfg.Custom("broken_sig_action", false, false,
		func() (interface{}, error) {
			return check.FailAction{}, nil
		}, check.FailActionDirective, &c.brokenSigAction)
	cfg.Custom("no_sig_action", false, false,
		func() (interface{}, error) {
			return check.FailAction{}, nil
		}, check.FailActionDirective, &c.noSigAction)
	cfg.Int32("ok_score", false, false, 0, &c.okScore)
	_, err := cfg.Process()
	if err != nil {
		return err
	}

	c.requiredFields = make(map[string]struct{})
	for _, field := range requiredFields {
		c.requiredFields[nettextproto.CanonicalMIMEHeaderKey(field)] = struct{}{}
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

func (d dkimCheckState) CheckConnection() module.CheckResult {
	return module.CheckResult{}
}

func (d dkimCheckState) CheckSender(mailFrom string) module.CheckResult {
	return module.CheckResult{}
}

func (d dkimCheckState) CheckRcpt(rcptTo string) module.CheckResult {
	return module.CheckResult{}
}

func (d dkimCheckState) CheckBody(header textproto.Header, body buffer.Buffer) module.CheckResult {
	if !header.Has("DKIM-Signature") {
		if d.c.noSigAction.Reject || d.c.noSigAction.Quarantine {
			d.log.Printf("no signatures present")
		} else {
			d.log.Debugf("no signatures present")
		}
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
		d.log.Println("can't open body:", err)
		return module.CheckResult{RejectErr: err}
	}

	verifications, err := dkim.Verify(io.MultiReader(&b, bodyRdr))
	if err != nil {
		d.log.Println("unexpected verification fail:", err)
		return module.CheckResult{RejectErr: err}
	}

	goodSigs := false

	res := module.CheckResult{AuthResult: make([]authres.Result, 0, len(verifications))}
	for _, verif := range verifications {
		var val authres.ResultValue
		if verif.Err == nil {
			goodSigs = true
			d.log.Debugf("good signature from %s (%s)", verif.Domain, verif.Identifier)
			val = authres.ResultPass
		} else {
			val = authres.ResultFail
			d.log.Printf("%v (domain = %s, identifier = %s)", strings.TrimPrefix(verif.Err.Error(), "dkim: "), verif.Domain, verif.Identifier)
			if dkim.IsPermFail(err) {
				val = authres.ResultPermError
			}
			if dkim.IsTempFail(err) {
				val = authres.ResultTempError
			}
		}

		signedFields := make(map[string]struct{}, len(verif.HeaderKeys))
		for _, field := range verif.HeaderKeys {
			signedFields[nettextproto.CanonicalMIMEHeaderKey(field)] = struct{}{}
		}
		for field := range d.c.requiredFields {
			if _, ok := signedFields[field]; !ok {
				// Brace-enclosed strings are comments that are allowed in Authentication-Results
				// field. Since go-msgauth does not allow us to insert them explicitly, we
				// "smuggle" them in Value field that then gets copied into resulting field
				// giving us "dkim=policy (some header fields are not signed)" which is what we want.
				val = authres.ResultPolicy + " (some header fields are not signed)"
			}
		}

		if verif.BodyLength >= 0 && !d.c.allowBodySubset {
			val = authres.ResultPolicy + " (body limit it used)"
		}

		res.AuthResult = append(res.AuthResult, &authres.DKIMResult{
			Value:      val,
			Domain:     verif.Domain,
			Identifier: verif.Identifier,
		})
	}

	if !goodSigs {
		return d.c.brokenSigAction.Apply(res)
	}
	res.ScoreAdjust = d.c.okScore
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
