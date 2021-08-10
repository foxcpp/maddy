/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package dkim

import (
	"bytes"
	"context"
	"errors"
	"io"
	nettextproto "net/textproto"
	"runtime/trace"
	"strings"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-msgauth/authres"
	"github.com/emersion/go-msgauth/dkim"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/dns"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/target"
)

type Check struct {
	instName string
	log      log.Logger

	requiredFields  map[string]struct{}
	brokenSigAction modconfig.FailAction
	noSigAction     modconfig.FailAction
	failOpen        bool

	resolver dns.Resolver
}

func New(_, instName string, _, inlineArgs []string) (module.Module, error) {
	if len(inlineArgs) != 0 {
		return nil, errors.New("check.dkim: inline arguments are not used")
	}
	return &Check{
		instName: instName,
		log:      log.Logger{Name: "check.dkim"},
		resolver: dns.DefaultResolver(),
	}, nil
}

func (c *Check) Init(cfg *config.Map) error {
	var requiredFields []string

	cfg.Bool("debug", true, false, &c.log.Debug)
	cfg.StringList("required_fields", false, false, []string{"From", "Subject"}, &requiredFields)
	cfg.Bool("fail_open", false, false, &c.failOpen)
	cfg.Custom("broken_sig_action", false, false,
		func() (interface{}, error) {
			return modconfig.FailAction{}, nil
		}, modconfig.FailActionDirective, &c.brokenSigAction)
	cfg.Custom("no_sig_action", false, false,
		func() (interface{}, error) {
			return modconfig.FailAction{}, nil
		}, modconfig.FailActionDirective, &c.noSigAction)
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
	return "check.dkim"
}

func (c *Check) InstanceName() string {
	return c.instName
}

type dkimCheckState struct {
	c       *Check
	msgMeta *module.MsgMetadata
	log     log.Logger
}

func (d *dkimCheckState) CheckConnection(ctx context.Context) module.CheckResult {
	return module.CheckResult{}
}

func (d *dkimCheckState) CheckSender(ctx context.Context, mailFrom string) module.CheckResult {
	return module.CheckResult{}
}

func (d *dkimCheckState) CheckRcpt(ctx context.Context, rcptTo string) module.CheckResult {
	return module.CheckResult{}
}

func (d *dkimCheckState) CheckBody(ctx context.Context, header textproto.Header, body buffer.Buffer) module.CheckResult {
	defer trace.StartRegion(ctx, "check.dkim/CheckBody").End()

	if !header.Has("DKIM-Signature") {
		if d.c.noSigAction.Reject || d.c.noSigAction.Quarantine {
			d.log.Printf("no signatures present")
		} else {
			d.log.Debugf("no signatures present")
		}
		return d.c.noSigAction.Apply(module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:         550,
				EnhancedCode: exterrors.EnhancedCode{5, 7, 20},
				Message:      "No DKIM signatures",
				CheckName:    "check.dkim",
			},
			AuthResult: []authres.Result{
				&authres.DKIMResult{
					Value: authres.ResultNone,
				},
			},
		})
	}

	b := bytes.Buffer{}
	_ = textproto.WriteHeader(&b, header)
	bodyRdr, err := body.Open()
	if err != nil {
		return module.CheckResult{
			Reject: true,
			Reason: exterrors.WithTemporary(
				exterrors.WithFields(err, map[string]interface{}{
					"check":    "check.dkim",
					"smtp_msg": "Internal I/O error",
				}),
				true,
			),
		}
	}

	verifications, err := dkim.VerifyWithOptions(io.MultiReader(&b, bodyRdr), &dkim.VerifyOptions{
		LookupTXT: func(domain string) ([]string, error) {
			return d.c.resolver.LookupTXT(ctx, domain)
		},
	})
	if err != nil {
		return module.CheckResult{
			Reject: true,
			Reason: exterrors.WithTemporary(
				exterrors.WithFields(err, map[string]interface{}{
					"check":    "check.dkim",
					"smtp_msg": "Internal error during policy check",
				}),
				true,
			),
		}
	}

	goodSigs := false

	res := module.CheckResult{AuthResult: make([]authres.Result, 0, len(verifications))}
	for _, verif := range verifications {
		val := authres.ResultValue(authres.ResultPass)
		reason := ""
		if verif.Err != nil {
			val = authres.ResultFail

			reason = strings.TrimPrefix(verif.Err.Error(), "dkim: ")
			if !d.c.brokenSigAction.Reject || !d.c.brokenSigAction.Quarantine {
				d.log.DebugMsg("bad signature", "domain", verif.Domain, "identifier", verif.Identifier)
			}
			if dkim.IsPermFail(verif.Err) {
				val = authres.ResultPermError
			}
			if dkim.IsTempFail(verif.Err) {
				if !d.c.failOpen {
					return module.CheckResult{
						Reject: true,
						Reason: &exterrors.SMTPError{
							Code:         421,
							EnhancedCode: exterrors.EnhancedCode{4, 7, 20},
							Message:      "Temporary error during DKIM verification",
							CheckName:    "check.dkim",
							Err:          verif.Err,
						},
					}
				}
				val = authres.ResultTempError
			}

			res.AuthResult = append(res.AuthResult, &authres.DKIMResult{
				Value:      val,
				Reason:     reason,
				Domain:     verif.Domain,
				Identifier: verif.Identifier,
			})
			continue
		}

		signedFields := make(map[string]struct{}, len(verif.HeaderKeys))
		for _, field := range verif.HeaderKeys {
			signedFields[nettextproto.CanonicalMIMEHeaderKey(field)] = struct{}{}
		}
		for field := range d.c.requiredFields {
			if _, ok := signedFields[field]; !ok {
				val = authres.ResultPermError
				reason = "some header fields are not signed"
			}
		}

		if val == authres.ResultPass {
			goodSigs = true
			d.log.DebugMsg("good signature", "domain", verif.Domain, "identifier", verif.Identifier)
		}

		res.AuthResult = append(res.AuthResult, &authres.DKIMResult{
			Value:      val,
			Reason:     reason,
			Domain:     verif.Domain,
			Identifier: verif.Identifier,
		})
	}

	if !goodSigs {
		res.Reason = &exterrors.SMTPError{
			Code:         550,
			EnhancedCode: exterrors.EnhancedCode{5, 7, 20},
			Message:      "No passing DKIM signatures",
			CheckName:    "check.dkim",
		}
		return d.c.brokenSigAction.Apply(res)
	}
	return res
}

func (d *dkimCheckState) Name() string {
	return "check.dkim"
}

func (d *dkimCheckState) Close() error {
	return nil
}

func (c *Check) CheckStateForMsg(ctx context.Context, msgMeta *module.MsgMetadata) (module.CheckState, error) {
	return &dkimCheckState{
		c:       c,
		msgMeta: msgMeta,
		log:     target.DeliveryLogger(c.log, msgMeta),
	}, nil
}

func init() {
	module.Register("check.dkim", New)
}
