package maddy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/module"
)

type SMTPPipelineStep interface {
	Pass(ctx *module.DeliveryContext, msg io.Reader) (io.Reader, bool, error)
}

type requireAuthStep struct{}

func (requireAuthStep) Pass(ctx *module.DeliveryContext, _ io.Reader) (io.Reader, bool, error) {
	if ctx.AuthUser != "" {
		return nil, true, nil
	}
	return nil, false, errors.New("non-anonymous authentication required")
}

type filterStep struct {
	f    module.Filter
	opts map[string]string
}

func (step filterStep) Pass(ctx *module.DeliveryContext, msg io.Reader) (io.Reader, bool, error) {
	ctx.Opts = step.opts
	r, err := step.f.Apply(ctx, msg)
	if err == module.ErrSilentDrop {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return r, true, nil
}

type deliveryStep struct {
	t    module.DeliveryTarget
	opts map[string]string
}

func (step deliveryStep) Pass(ctx *module.DeliveryContext, msg io.Reader) (io.Reader, bool, error) {
	err := step.t.Deliver(*ctx, msg)
	return nil, err == nil, err
}

type matchStep struct {
	field    string
	pattern  string
	inverted bool
	regexp   *regexp.Regexp
	substeps []SMTPPipelineStep
}

func (step matchStep) Pass(ctx *module.DeliveryContext, msg io.Reader) (io.Reader, bool, error) {
	match := false
	switch step.field {
	case "rcpt":
		for _, rcpt := range ctx.To {
			if step.matches(rcpt) {
				match = true
				break
			}
		}
	case "from":
		match = step.matches(ctx.From)
	case "src_addr":
		match = step.matches(ctx.SrcAddr.String())
	case "src_hostname":
		match = step.matches(ctx.SrcHostname)
	}

	if !match {
		return nil, true, nil
	}

	currentMsg := msg
	var buf bytes.Buffer
	for _, substep := range step.substeps {
		r, cont, err := substep.Pass(ctx, currentMsg)
		if !cont {
			return nil, cont, err
		}

		if r != nil {
			if _, err := io.Copy(&buf, r); err != nil {
				return nil, false, err
			}
			currentMsg = &buf
		}
	}

	if currentMsg != msg {
		return currentMsg, true, nil
	}
	return msg, true, nil
}

func (step matchStep) matches(s string) bool {
	var match bool
	if step.regexp != nil {
		match = step.regexp.MatchString(s)
	} else {
		match = strings.Contains(s, step.pattern)
	}

	if step.inverted {
		return !match
	} else {
		return match
	}
}

type stopStep struct{}

func (stopStep) Pass(_ *module.DeliveryContext, _ io.Reader) (io.Reader, bool, error) {
	return nil, false, nil
}

func filterStepFromCfg(node config.Node) (SMTPPipelineStep, error) {
	if len(node.Args) == 0 {
		return nil, errors.New("filter: expected at least one argument")
	}

	modInst := module.GetInstance(node.Args[0])
	if modInst == nil {
		return nil, fmt.Errorf("filter: unknown module instance: %s", node.Args[0])
	}

	filterInst, ok := modInst.(module.Filter)
	if !ok {
		return nil, fmt.Errorf("filter: module %s (%s) doesn't implements Filter", modInst.Name(), modInst.InstanceName())
	}

	return filterStep{
		f:    filterInst,
		opts: readOpts(node.Args[1:]),
	}, nil
}

func deliveryStepFromCfg(node config.Node) (SMTPPipelineStep, error) {
	if len(node.Args) == 0 {
		return nil, errors.New("delivery: expected at least one argument")
	}

	modInst := module.GetInstance(node.Args[0])
	if modInst == nil {
		return nil, fmt.Errorf("delivery: unknown module instance: %s", node.Args[0])
	}

	deliveryInst, ok := modInst.(module.DeliveryTarget)
	if !ok {
		return nil, fmt.Errorf("delivery: module %s (%s) doesn't implements DeliveryTarget", modInst.Name(), modInst.InstanceName())
	}

	return deliveryStep{
		t:    deliveryInst,
		opts: readOpts(node.Args[1:]),
	}, nil
}

func readOpts(args []string) (res map[string]string) {
	res = make(map[string]string)
	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 1)
		if len(parts) == 1 {
			res[parts[0]] = ""
		} else {
			res[parts[0]] = parts[1]
		}

	}
	return
}

func matchStepFromCfg(node config.Node) (SMTPPipelineStep, error) {
	if len(node.Args) != 3 && len(node.Args) != 2 {
		return nil, errors.New("match: expected 3 or 2 arguments")
	}

	res := matchStep{}

	if len(node.Args) == 3 {
		if node.Args[0] != "no" {
			return nil, errors.New("expected 'no' after match")
		}
		res.inverted = true
		node.Args = node.Args[1:]
	}

	res.field = node.Args[0]
	res.pattern = node.Args[1]
	if strings.HasPrefix(res.pattern, "/") && strings.HasSuffix(res.pattern, "/") {
		res.pattern = res.pattern[1 : len(res.pattern)-1]
		var err error
		res.regexp, err = regexp.Compile(res.pattern)
		if err != nil {
			return nil, err
		}
	}

	for _, child := range node.Children {
		step, err := StepFromCfg(child)
		if err != nil {
			return nil, err
		}
		res.substeps = append(res.substeps, step)
	}

	return res, nil
}

func StepFromCfg(node config.Node) (SMTPPipelineStep, error) {
	switch node.Name {
	case "filter":
		return filterStepFromCfg(node)
	case "delivery":
		return deliveryStepFromCfg(node)
	case "match":
		return matchStepFromCfg(node)
	case "stop":
		return stopStep{}, nil
	case "require_auth":
		return requireAuthStep{}, nil
	default:
		return nil, fmt.Errorf("unknown pipeline step: %s", node.Name)
	}
}
