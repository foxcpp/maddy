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

package modconfig

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/module"
)

// FailAction specifies actions that messages pipeline should take based on the
// result of the check.
//
// Its check module responsibility to apply FailAction on the CheckResult it
// returns. It is intended to be used as follows:
//
// Add the configuration directive to allow user to specify the action:
//     cfg.Custom("SOME_action", false, false,
//     	func() (interface{}, error) {
//     		return modconfig.FailAction{Quarantine: true}, nil
//     	}, modconfig.FailActionDirective, &yourModule.SOMEAction)
// return in func literal is the default value, you might want to adjust it.
//
// Call yourModule.SOMEAction.Apply on CheckResult containing only the
// Reason field:
//     func (yourModule YourModule) CheckConnection() module.CheckResult {
//         return yourModule.SOMEAction.Apply(module.CheckResult{
//             Reason: ...,
//         })
//     }
type FailAction struct {
	Quarantine bool
	Reject     bool

	ReasonOverride *exterrors.SMTPError
}

func FailActionDirective(_ *config.Map, node config.Node) (interface{}, error) {
	if len(node.Children) != 0 {
		return nil, config.NodeErr(node, "can't declare block here")
	}

	val, err := ParseActionDirective(node.Args)
	if err != nil {
		return nil, config.NodeErr(node, "%v", err)
	}
	return val, nil
}

func ParseActionDirective(args []string) (FailAction, error) {
	if len(args) == 0 {
		return FailAction{}, errors.New("expected at least 1 argument")
	}

	res := FailAction{}

	switch args[0] {
	case "reject", "quarantine":
		if len(args) > 1 {
			var err error
			res.ReasonOverride, err = ParseRejectDirective(args[1:])
			if err != nil {
				return FailAction{}, err
			}
		}
	case "ignore":
	default:
		return FailAction{}, errors.New("invalid action")
	}

	res.Reject = args[0] == "reject"
	res.Quarantine = args[0] == "quarantine"
	return res, nil
}

// Apply merges the result of check execution with action configuration specified
// in the check configuration.
func (cfa FailAction) Apply(originalRes module.CheckResult) module.CheckResult {
	if originalRes.Reason == nil {
		return originalRes
	}

	if cfa.ReasonOverride != nil {
		// Wrap instead of replace to preserve other fields.
		originalRes.Reason = &exterrors.SMTPError{
			Code:         cfa.ReasonOverride.Code,
			EnhancedCode: cfa.ReasonOverride.EnhancedCode,
			Message:      cfa.ReasonOverride.Message,
			Err:          originalRes.Reason,
		}
	}

	originalRes.Quarantine = cfa.Quarantine || originalRes.Quarantine
	originalRes.Reject = cfa.Reject || originalRes.Reject
	return originalRes
}

func ParseRejectDirective(args []string) (*exterrors.SMTPError, error) {
	code := 554
	enchCode := exterrors.EnhancedCode{0, 7, 0}
	msg := "Message rejected due to a local policy"
	var err error
	switch len(args) {
	case 3:
		msg = args[2]
		if msg == "" {
			return nil, fmt.Errorf("message can't be empty")
		}
		fallthrough
	case 2:
		enchCode, err = parseEnhancedCode(args[1])
		if err != nil {
			return nil, err
		}
		if enchCode[0] != 4 && enchCode[0] != 5 {
			return nil, fmt.Errorf("enhanced code should use either 4 or 5 as a first number")
		}
		fallthrough
	case 1:
		code, err = strconv.Atoi(args[0])
		if err != nil {
			return nil, fmt.Errorf("invalid error code integer: %v", err)
		}
		if (code/100) != 4 && (code/100) != 5 {
			return nil, fmt.Errorf("error code should start with either 4 or 5")
		}
		// If enchanced code is not set - set first digit based on provided "basic" code.
		if enchCode[0] == 0 {
			enchCode[0] = code / 100
		}
	case 0:
		// If no codes provided at all - use 5.7.0 and 554.
		enchCode[0] = 5
	default:
		return nil, fmt.Errorf("invalid count of arguments")
	}
	return &exterrors.SMTPError{
		Code:         code,
		EnhancedCode: enchCode,
		Message:      msg,
		Reason:       "reject directive used",
	}, nil
}

func parseEnhancedCode(s string) (exterrors.EnhancedCode, error) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return exterrors.EnhancedCode{}, fmt.Errorf("wrong amount of enhanced code parts")
	}

	code := exterrors.EnhancedCode{}
	for i, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil {
			return code, err
		}
		code[i] = num
	}
	return code, nil
}
