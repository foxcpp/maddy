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

package msgpipeline

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/foxcpp/maddy/framework/address"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/dns"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/modify"
)

type sourceIn struct {
	t     module.Table
	block sourceBlock
}

type msgpipelineCfg struct {
	globalChecks    []module.Check
	globalModifiers modify.Group
	sourceIn        []sourceIn
	perSource       map[string]sourceBlock
	defaultSource   sourceBlock
	doDMARC         bool
}

func parseMsgPipelineRootCfg(globals map[string]interface{}, nodes []config.Node) (msgpipelineCfg, error) {
	cfg := msgpipelineCfg{
		perSource: map[string]sourceBlock{},
	}
	var defaultSrcRaw []config.Node
	var othersRaw []config.Node
	for _, node := range nodes {
		switch node.Name {
		case "check":
			globalChecks, err := parseChecksGroup(globals, node)
			if err != nil {
				return msgpipelineCfg{}, err
			}

			cfg.globalChecks = append(cfg.globalChecks, globalChecks...)
		case "modify":
			globalModifiers, err := parseModifiersGroup(globals, node)
			if err != nil {
				return msgpipelineCfg{}, err
			}

			cfg.globalModifiers.Modifiers = append(cfg.globalModifiers.Modifiers, globalModifiers.Modifiers...)
		case "source_in":
			var tbl module.Table
			if err := modconfig.ModuleFromNode("table", node.Args, config.Node{}, globals, &tbl); err != nil {
				return msgpipelineCfg{}, err
			}
			srcBlock, err := parseMsgPipelineSrcCfg(globals, node.Children)
			if err != nil {
				return msgpipelineCfg{}, err
			}
			cfg.sourceIn = append(cfg.sourceIn, sourceIn{
				t:     tbl,
				block: srcBlock,
			})
		case "source":
			srcBlock, err := parseMsgPipelineSrcCfg(globals, node.Children)
			if err != nil {
				return msgpipelineCfg{}, err
			}

			if len(node.Args) == 0 {
				return msgpipelineCfg{}, config.NodeErr(node, "expected at least one source matching rule")
			}

			for _, rule := range node.Args {
				if strings.Contains(rule, "@") {
					rule, err = address.ForLookup(rule)
				} else {
					rule, err = dns.ForLookup(rule)
				}
				if err != nil {
					return msgpipelineCfg{}, config.NodeErr(node, "invalid source match rule: %v: %v", rule, err)
				}

				if !validMatchRule(rule) {
					return msgpipelineCfg{}, config.NodeErr(node, "invalid source routing rule: %v", rule)
				}

				if _, ok := cfg.perSource[rule]; ok {
					continue
				}

				cfg.perSource[rule] = srcBlock
			}
		case "default_source":
			if defaultSrcRaw != nil {
				return msgpipelineCfg{}, config.NodeErr(node, "duplicate 'default_source' block")
			}
			defaultSrcRaw = node.Children
		case "dmarc":
			switch len(node.Args) {
			case 1:
				switch node.Args[0] {
				case "yes":
					cfg.doDMARC = true
				case "no":
				default:
					return msgpipelineCfg{}, config.NodeErr(node, "invalid argument for dmarc")
				}
			case 0:
				cfg.doDMARC = true
			}
		case "deliver_to", "reroute", "destination_in", "destination", "default_destination", "reject":
			othersRaw = append(othersRaw, node)
		default:
			return msgpipelineCfg{}, config.NodeErr(node, "unknown pipeline directive: %s", node.Name)
		}
	}

	if len(cfg.perSource) == 0 && len(defaultSrcRaw) == 0 {
		if len(othersRaw) == 0 {
			return msgpipelineCfg{}, fmt.Errorf("empty pipeline configuration, use 'reject' to reject messages")
		}

		var err error
		cfg.defaultSource, err = parseMsgPipelineSrcCfg(globals, othersRaw)
		return cfg, err
	} else if len(othersRaw) != 0 {
		return msgpipelineCfg{}, config.NodeErr(othersRaw[0], "can't put handling directives together with source rules, did you mean to put it into 'default_source' block or into all source blocks?")
	}

	if len(defaultSrcRaw) == 0 {
		return msgpipelineCfg{}, config.NodeErr(nodes[0], "missing or empty default source block, use default_source { reject } to reject messages")
	}

	var err error
	cfg.defaultSource, err = parseMsgPipelineSrcCfg(globals, defaultSrcRaw)
	return cfg, err
}

func parseMsgPipelineSrcCfg(globals map[string]interface{}, nodes []config.Node) (sourceBlock, error) {
	src := sourceBlock{
		perRcpt: map[string]*rcptBlock{},
	}
	var defaultRcptRaw []config.Node
	var othersRaw []config.Node
	for _, node := range nodes {
		switch node.Name {
		case "check":
			checks, err := parseChecksGroup(globals, node)
			if err != nil {
				return sourceBlock{}, err
			}

			src.checks = append(src.checks, checks...)
		case "modify":
			modifiers, err := parseModifiersGroup(globals, node)
			if err != nil {
				return sourceBlock{}, err
			}

			src.modifiers.Modifiers = append(src.modifiers.Modifiers, modifiers.Modifiers...)
		case "destination_in":
			var tbl module.Table
			if err := modconfig.ModuleFromNode("table", node.Args, config.Node{}, globals, &tbl); err != nil {
				return sourceBlock{}, err
			}
			rcptBlock, err := parseMsgPipelineRcptCfg(globals, node.Children)
			if err != nil {
				return sourceBlock{}, err
			}
			src.rcptIn = append(src.rcptIn, rcptIn{
				t:     tbl,
				block: rcptBlock,
			})
		case "destination":
			rcptBlock, err := parseMsgPipelineRcptCfg(globals, node.Children)
			if err != nil {
				return sourceBlock{}, err
			}

			if len(node.Args) == 0 {
				return sourceBlock{}, config.NodeErr(node, "expected at least one destination match rule")
			}

			for _, rule := range node.Args {
				if strings.Contains(rule, "@") {
					rule, err = address.ForLookup(rule)
				} else {
					rule, err = dns.ForLookup(rule)
				}
				if err != nil {
					return sourceBlock{}, config.NodeErr(node, "invalid destination match rule: %v: %v", rule, err)
				}

				if !validMatchRule(rule) {
					return sourceBlock{}, config.NodeErr(node, "invalid destination match rule: %v", rule)
				}

				if _, ok := src.perRcpt[rule]; ok {
					continue
				}

				src.perRcpt[rule] = rcptBlock
			}
		case "default_destination":
			if defaultRcptRaw != nil {
				return sourceBlock{}, config.NodeErr(node, "duplicate 'default_destination' block")
			}
			defaultRcptRaw = node.Children
		case "deliver_to", "reroute", "reject":
			othersRaw = append(othersRaw, node)
		default:
			return sourceBlock{}, config.NodeErr(node, "unknown pipeline directive: %s", node.Name)
		}
	}

	if len(src.perRcpt) == 0 && len(defaultRcptRaw) == 0 {
		if len(othersRaw) == 0 {
			return sourceBlock{}, fmt.Errorf("empty source block, use 'reject' to reject messages")
		}

		var err error
		src.defaultRcpt, err = parseMsgPipelineRcptCfg(globals, othersRaw)
		return src, err
	} else if len(othersRaw) != 0 {
		return sourceBlock{}, config.NodeErr(othersRaw[0], "can't put handling directives together with destination rules, did you mean to put it into 'default' block or into all recipient blocks?")
	}

	if len(defaultRcptRaw) == 0 {
		return sourceBlock{}, config.NodeErr(nodes[0], "missing or empty default destination block, use default_destination { reject } to reject messages")
	}

	var err error
	src.defaultRcpt, err = parseMsgPipelineRcptCfg(globals, defaultRcptRaw)
	return src, err
}

func parseMsgPipelineRcptCfg(globals map[string]interface{}, nodes []config.Node) (*rcptBlock, error) {
	rcpt := rcptBlock{}
	for _, node := range nodes {
		switch node.Name {
		case "check":
			checks, err := parseChecksGroup(globals, node)
			if err != nil {
				return nil, err
			}

			rcpt.checks = append(rcpt.checks, checks...)
		case "modify":
			modifiers, err := parseModifiersGroup(globals, node)
			if err != nil {
				return nil, err
			}

			rcpt.modifiers.Modifiers = append(rcpt.modifiers.Modifiers, modifiers.Modifiers...)
		case "deliver_to":
			if rcpt.rejectErr != nil {
				return nil, config.NodeErr(node, "can't use 'reject' and 'deliver_to' together")
			}

			if len(node.Args) == 0 {
				return nil, config.NodeErr(node, "required at least one argument")
			}
			mod, err := modconfig.DeliveryTarget(globals, node.Args, node)
			if err != nil {
				return nil, err
			}

			rcpt.targets = append(rcpt.targets, mod)
		case "reroute":
			if len(node.Children) == 0 {
				return nil, config.NodeErr(node, "missing or empty reroute pipeline configuration")
			}

			pipeline, err := New(globals, node.Children)
			if err != nil {
				return nil, err
			}

			rcpt.targets = append(rcpt.targets, pipeline)
		case "reject":
			if len(rcpt.targets) != 0 {
				return nil, config.NodeErr(node, "can't use 'reject' and 'deliver_to' together")
			}

			var err error
			rcpt.rejectErr, err = parseRejectDirective(node)
			if err != nil {
				return nil, err
			}
		default:
			return nil, config.NodeErr(node, "invalid directive")
		}
	}
	return &rcpt, nil
}

func parseRejectDirective(node config.Node) (*exterrors.SMTPError, error) {
	code := 554
	enchCode := exterrors.EnhancedCode{5, 7, 0}
	msg := "Message rejected due to a local policy"
	var err error
	switch len(node.Args) {
	case 3:
		msg = node.Args[2]
		if msg == "" {
			return nil, config.NodeErr(node, "message can't be empty")
		}
		fallthrough
	case 2:
		enchCode, err = parseEnhancedCode(node.Args[1])
		if err != nil {
			return nil, config.NodeErr(node, "%v", err)
		}
		if enchCode[0] != 4 && enchCode[0] != 5 {
			return nil, config.NodeErr(node, "enhanced code should use either 4 or 5 as a first number")
		}
		fallthrough
	case 1:
		code, err = strconv.Atoi(node.Args[0])
		if err != nil {
			return nil, config.NodeErr(node, "invalid error code integer: %v", err)
		}
		if (code/100) != 4 && (code/100) != 5 {
			return nil, config.NodeErr(node, "error code should start with either 4 or 5")
		}
	case 0:
	default:
		return nil, config.NodeErr(node, "invalid count of arguments")
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

func parseChecksGroup(globals map[string]interface{}, node config.Node) ([]module.Check, error) {
	var cg *CheckGroup
	err := modconfig.GroupFromNode("checks", node.Args, node, globals, &cg)
	if err != nil {
		return nil, err
	}
	return cg.L, nil
}

func parseModifiersGroup(globals map[string]interface{}, node config.Node) (modify.Group, error) {
	// Module object is *modify.Group, not modify.Group.
	var mg *modify.Group
	err := modconfig.GroupFromNode("modifiers", node.Args, node, globals, &mg)
	if err != nil {
		return modify.Group{}, err
	}
	return *mg, nil
}

func validMatchRule(rule string) bool {
	return address.ValidDomain(rule) || address.Valid(rule)
}
