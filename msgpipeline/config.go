package msgpipeline

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/foxcpp/maddy/address"
	"github.com/foxcpp/maddy/config"
	modconfig "github.com/foxcpp/maddy/config/module"
	"github.com/foxcpp/maddy/exterrors"
	"github.com/foxcpp/maddy/modify"
	"github.com/foxcpp/maddy/module"
)

type msgpipelineCfg struct {
	globalChecks    []module.Check
	globalModifiers modify.Group
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
			if len(node.Children) == 0 {
				return msgpipelineCfg{}, config.NodeErr(&node, "empty checks block")
			}

			var err error
			cfg.globalChecks, err = parseChecksGroup(globals, node.Children)
			if err != nil {
				return msgpipelineCfg{}, err
			}
		case "modify":
			if len(node.Children) == 0 {
				return msgpipelineCfg{}, config.NodeErr(&node, "empty modifiers block")
			}

			var err error
			cfg.globalModifiers, err = parseModifiersGroup(globals, node.Children)
			if err != nil {
				return msgpipelineCfg{}, err
			}
		case "source":
			srcBlock, err := parseMsgPipelineSrcCfg(globals, node.Children)
			if err != nil {
				return msgpipelineCfg{}, err
			}

			if len(node.Args) == 0 {
				return msgpipelineCfg{}, config.NodeErr(&node, "expected at least one source matching rule")
			}

			for _, rule := range node.Args {
				if !validMatchRule(rule) {
					return msgpipelineCfg{}, config.NodeErr(&node, "invalid source routing rule: %v", rule)
				}

				cfg.perSource[rule] = srcBlock
			}
		case "default_source":
			if defaultSrcRaw != nil {
				return msgpipelineCfg{}, config.NodeErr(&node, "duplicate 'default_source' block")
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
					return msgpipelineCfg{}, config.NodeErr(&node, "invalid argument for dmarc")
				}
			case 0:
				cfg.doDMARC = true
			}
		default:
			othersRaw = append(othersRaw, node)
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
		return msgpipelineCfg{}, config.NodeErr(&othersRaw[0], "can't put handling directives together with source rules, did you mean to put it into 'default_source' block or into all source blocks?")
	}

	if len(defaultSrcRaw) == 0 {
		return msgpipelineCfg{}, config.NodeErr(&nodes[0], "missing or empty default source block, use 'reject' to reject messages")
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
			if len(node.Children) == 0 {
				return sourceBlock{}, config.NodeErr(&node, "empty checks block")
			}

			var err error
			src.checks, err = parseChecksGroup(globals, node.Children)
			if err != nil {
				return sourceBlock{}, err
			}
		case "modify":
			if len(node.Children) == 0 {
				return sourceBlock{}, config.NodeErr(&node, "empty modifiers block")
			}

			var err error
			src.modifiers, err = parseModifiersGroup(globals, node.Children)
			if err != nil {
				return sourceBlock{}, err
			}
		case "destination":
			rcptBlock, err := parseMsgPipelineRcptCfg(globals, node.Children)
			if err != nil {
				return sourceBlock{}, err
			}

			if len(node.Args) == 0 {
				return sourceBlock{}, config.NodeErr(&node, "expected at least one destination match rule")
			}

			for _, rule := range node.Args {
				if !validMatchRule(rule) {
					return sourceBlock{}, config.NodeErr(&node, "invalid destination match rule: %v", rule)
				}

				src.perRcpt[rule] = rcptBlock
			}
		case "default_destination":
			if defaultRcptRaw != nil {
				return sourceBlock{}, config.NodeErr(&node, "duplicate 'default_destination' block")
			}
			defaultRcptRaw = node.Children
		default:
			othersRaw = append(othersRaw, node)
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
		return sourceBlock{}, config.NodeErr(&othersRaw[0], "can't put handling directives together with destination rules, did you mean to put it into 'default' block or into all recipient blocks?")
	}

	if len(defaultRcptRaw) == 0 {
		return sourceBlock{}, config.NodeErr(&nodes[0], "missing or empty default source block, use 'reject' to reject messages")
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
			if len(node.Children) == 0 {
				return nil, config.NodeErr(&node, "empty checks block")
			}

			var err error
			rcpt.checks, err = parseChecksGroup(globals, node.Children)
			if err != nil {
				return nil, err
			}
		case "modify":
			if len(node.Children) == 0 {
				return nil, config.NodeErr(&node, "empty modifiers block")
			}

			var err error
			rcpt.modifiers, err = parseModifiersGroup(globals, node.Children)
			if err != nil {
				return nil, err
			}
		case "deliver_to":
			if rcpt.rejectErr != nil {
				return nil, config.NodeErr(&node, "can't use 'reject' and 'deliver_to' together")
			}

			if len(node.Args) == 0 {
				return nil, config.NodeErr(&node, "required at least one argument")
			}
			mod, err := modconfig.DeliveryTarget(globals, node.Args, &node)
			if err != nil {
				return nil, err
			}

			rcpt.targets = append(rcpt.targets, mod)
		case "reject":
			if len(rcpt.targets) != 0 {
				return nil, config.NodeErr(&node, "can't use 'reject' and 'deliver_to' together")
			}

			var err error
			rcpt.rejectErr, err = parseRejectDirective(node)
			if err != nil {
				return nil, err
			}
		default:
			return nil, config.NodeErr(&node, "invalid directive")
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
			return nil, config.NodeErr(&node, "message can't be empty")
		}
		fallthrough
	case 2:
		enchCode, err = parseEnhancedCode(node.Args[1])
		if err != nil {
			return nil, config.NodeErr(&node, "%v", err)
		}
		if enchCode[0] != 4 && enchCode[0] != 5 {
			return nil, config.NodeErr(&node, "enhanced code should use either 4 or 5 as a first number")
		}
		fallthrough
	case 1:
		code, err = strconv.Atoi(node.Args[0])
		if err != nil {
			return nil, config.NodeErr(&node, "invalid error code integer: %v", err)
		}
		if (code/100) != 4 && (code/100) != 5 {
			return nil, config.NodeErr(&node, "error code should start with either 4 or 5")
		}
	case 0:
	default:
		return nil, config.NodeErr(&node, "invalid count of arguments")
	}
	return &exterrors.SMTPError{
		Code:         code,
		EnhancedCode: enchCode,
		Message:      msg,
		Misc: map[string]interface{}{
			"reason": "reject directive used",
		},
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

func parseChecksGroup(globals map[string]interface{}, nodes []config.Node) ([]module.Check, error) {
	checks := make([]module.Check, 0, len(nodes))
	for _, child := range nodes {
		msgCheck, err := modconfig.MessageCheck(globals, append([]string{child.Name}, child.Args...), &child)
		if err != nil {
			return nil, err
		}

		checks = append(checks, msgCheck)
	}
	return checks, nil
}

func parseModifiersGroup(globals map[string]interface{}, nodes []config.Node) (modify.Group, error) {
	modifiers := modify.Group{}
	for _, child := range nodes {
		modifier, err := modconfig.MsgModifier(globals, append([]string{child.Name}, child.Args...), &child)
		if err != nil {
			return modify.Group{}, err
		}

		modifiers.Modifiers = append(modifiers.Modifiers, modifier)
	}
	return modifiers, nil
}

func validMatchRule(rule string) bool {
	return address.ValidDomain(rule) || address.Valid(rule)
}
