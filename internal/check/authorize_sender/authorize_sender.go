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

package authorize_sender

import (
	"context"
	"fmt"
	"net/mail"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/authz"
	"github.com/foxcpp/maddy/internal/table"
	"github.com/foxcpp/maddy/internal/target"
)

const modName = "check.authorize_sender"

type Check struct {
	instName string
	log      log.Logger

	checkHeader  bool
	emailPrepare module.Table
	userToEmail  module.Table

	unauthAction  modconfig.FailAction
	noMatchAction modconfig.FailAction
	errAction     modconfig.FailAction

	fromNorm func(string) (string, error)
	authNorm func(string) (string, error)
}

func New(_, instName string, _, inlineArgs []string) (module.Module, error) {
	return &Check{
		instName: instName,
	}, nil
}

func (c *Check) Name() string {
	return modName
}

func (c *Check) InstanceName() string {
	return c.instName
}

func (c *Check) Init(cfg *config.Map) error {
	cfg.Bool("debug", true, false, &c.log.Debug)

	cfg.Bool("check_header", false, true, &c.checkHeader)

	cfg.Custom("prepare_email", false, false, func() (interface{}, error) {
		return &table.Identity{}, nil
	}, modconfig.TableDirective, &c.emailPrepare)
	cfg.Custom("user_to_email", false, false, func() (interface{}, error) {
		return &table.Identity{}, nil
	}, modconfig.TableDirective, &c.userToEmail)

	cfg.Custom("unauth_action", false, false, func() (interface{}, error) {
		return modconfig.FailAction{Reject: true}, nil
	}, modconfig.FailActionDirective, &c.unauthAction)
	cfg.Custom("no_match_action", false, false, func() (interface{}, error) {
		return modconfig.FailAction{Reject: true}, nil
	}, modconfig.FailActionDirective, &c.noMatchAction)
	cfg.Custom("err_action", false, false, func() (interface{}, error) {
		return modconfig.FailAction{Reject: true}, nil
	}, modconfig.FailActionDirective, &c.errAction)

	var (
		authNormalize string
		fromNormalize string
		ok            bool
	)
	cfg.String("auth_normalize", false, false,
		"precis_casefold_email", &authNormalize)
	cfg.String("from_normalize", false, false,
		"precis_casefold_email", &fromNormalize)

	if _, err := cfg.Process(); err != nil {
		return err
	}

	c.authNorm, ok = authz.NormalizeFuncs[authNormalize]
	if !ok {
		return fmt.Errorf("%v: unknown normalization function: %v", modName, authNormalize)
	}
	c.fromNorm, ok = authz.NormalizeFuncs[fromNormalize]
	if !ok {
		return fmt.Errorf("%v: unknown normalization function: %v", modName, fromNormalize)
	}

	return nil
}

type state struct {
	c       *Check
	msgMeta *module.MsgMetadata
	log     log.Logger
}

func (c *Check) CheckStateForMsg(_ context.Context, msgMeta *module.MsgMetadata) (module.CheckState, error) {
	return &state{
		c:       c,
		msgMeta: msgMeta,
		log:     target.DeliveryLogger(c.log, msgMeta),
	}, nil
}

func (s *state) authzSender(ctx context.Context, authName, email string) module.CheckResult {
	if authName == "" {
		return s.c.unauthAction.Apply(module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:         530,
				EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
				Message:      "Authentication required",
				CheckName:    modName,
			}})
	}

	fromEmailNorm, err := s.c.fromNorm(email)
	if err != nil {
		return s.c.errAction.Apply(module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:         553,
				EnhancedCode: exterrors.EnhancedCode{5, 1, 7},
				Message:      "Unable to normalize sender address",
				CheckName:    modName,
				Err:          err,
			}})
	}
	authNameNorm, err := s.c.authNorm(authName)
	if err != nil {
		return s.c.errAction.Apply(module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:         535,
				EnhancedCode: exterrors.EnhancedCode{5, 7, 8},
				Message:      "Unable to normalize authorization username",
				CheckName:    modName,
			}})
	}

	var preparedEmail []string
	var ok bool
	s.log.DebugMsg("normalized names", "from", fromEmailNorm, "auth", authNameNorm)
	if emailPrepareMulti, isMulti := s.c.emailPrepare.(module.MultiTable); isMulti {
		preparedEmail, err = emailPrepareMulti.LookupMulti(ctx, fromEmailNorm)
		ok = len(preparedEmail) > 0
	} else {
		var preparedEmail_single string
		preparedEmail_single, ok, err = s.c.emailPrepare.Lookup(ctx, fromEmailNorm)
		preparedEmail = []string{preparedEmail_single}
	}
	s.log.DebugMsg("authorized emails", "preparedEmail", preparedEmail, "ok", ok)
	if err != nil {
		return s.c.errAction.Apply(module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:         454,
				EnhancedCode: exterrors.EnhancedCode{4, 7, 0},
				Message:      "Internal error during policy check",
				CheckName:    modName,
				Err:          err,
			}})
	}
	if !ok {
		preparedEmail = []string{fromEmailNorm}
	}

	ok, err = authz.AuthorizeEmailUse(ctx, authNameNorm, preparedEmail, s.c.userToEmail)
	if err != nil {
		return s.c.errAction.Apply(module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:         454,
				EnhancedCode: exterrors.EnhancedCode{4, 7, 0},
				Message:      "Internal error during policy check",
				CheckName:    modName,
				Err:          err,
			}})
	}
	if !ok {
		return s.c.noMatchAction.Apply(module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:         553,
				EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
				Message:      "Unauthorized use of sender address",
				CheckName:    modName,
			}})
	}

	return module.CheckResult{}
}

func (s *state) CheckConnection(_ context.Context) module.CheckResult {
	return module.CheckResult{}
}

func (s *state) CheckSender(ctx context.Context, fromEmail string) module.CheckResult {
	if s.msgMeta.Conn == nil {
		s.log.Msg("skipping locally generated message")
		return module.CheckResult{}
	}
	authName := s.msgMeta.Conn.AuthUser

	return s.authzSender(ctx, authName, fromEmail)
}

func (s *state) CheckRcpt(_ context.Context, _ string) module.CheckResult {
	return module.CheckResult{}
}

func (s *state) CheckBody(ctx context.Context, hdr textproto.Header, _ buffer.Buffer) module.CheckResult {
	if !s.c.checkHeader {
		return module.CheckResult{}
	}
	if s.msgMeta.Conn == nil {
		s.log.Msg("skipping locally generated message")
		return module.CheckResult{}
	}
	authName := s.msgMeta.Conn.AuthUser

	fromHdr := hdr.Get("From")
	if fromHdr == "" {
		return s.c.errAction.Apply(module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:         550,
				EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
				Message:      "Missing From header",
				CheckName:    modName,
			}})
	}
	list, err := mail.ParseAddressList(fromHdr)
	if err != nil || len(list) == 0 {
		return s.c.errAction.Apply(module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:         550,
				EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
				Message:      "Malformed From header",
				CheckName:    modName,
				Err:          err,
			}})
	}
	fromEmail := list[0].Address
	if len(list) > 1 {
		return s.c.errAction.Apply(module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:         550,
				EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
				Message:      "Multiple From addresses are not allowed",
				CheckName:    modName,
				Err:          err,
			}})
	}

	var senderAddr string
	if senderHdr := hdr.Get("Sender"); senderHdr != "" {
		sender, err := mail.ParseAddress(senderHdr)
		if err != nil {
			return s.c.errAction.Apply(module.CheckResult{
				Reason: &exterrors.SMTPError{
					Code:         550,
					EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
					Message:      "Malformed Sender header",
					CheckName:    modName,
					Err:          err,
				}})
		}
		senderAddr = sender.Address
	}

	res := s.authzSender(ctx, authName, fromEmail)
	if res.Reason == nil {
		return res
	}

	if senderAddr != "" && senderAddr != fromEmail {
		res = s.authzSender(ctx, authName, senderAddr)
		if res.Reason == nil {
			return res
		}
	}

	// Neither matched.
	return s.c.noMatchAction.Apply(module.CheckResult{
		Reason: &exterrors.SMTPError{
			Code:         553,
			EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
			Message:      "Unauthorized use of sender address",
			CheckName:    modName,
		}})
}

func (s *state) Close() error {
	return nil
}

func init() {
	module.Register(modName, New)
}
