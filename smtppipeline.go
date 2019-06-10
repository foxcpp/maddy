package maddy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/log"
	"github.com/emersion/maddy/module"
)

// TODO: Consider merging SMTPPipelineStep interface with module.Filter.

type SMTPPipelineStep interface {
	// Pass applies step's processing logic to the message.
	//
	// If Pass returns non-nil io.Reader - it should contain new message body.
	// If Pass returns false - message processing will be stopped, if err is
	// not nil - it will be returned to message source.
	Pass(ctx *module.DeliveryContext, body io.Reader) (newBody io.Reader, continueProcessing bool, err error)
}

type requireAuthStep struct{}

func (requireAuthStep) Pass(ctx *module.DeliveryContext, _ io.Reader) (io.Reader, bool, error) {
	if ctx.AuthUser != "" {
		return nil, true, nil
	}
	return nil, false, &smtp.SMTPError{
		Code:    550,
		Message: "Authentication is required",
	}
}

type filterStep struct {
	f module.Filter
}

func (step filterStep) Pass(ctx *module.DeliveryContext, msg io.Reader) (io.Reader, bool, error) {
	r, err := step.f.Apply(ctx, msg)
	if err == module.ErrSilentDrop {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return r, true, nil
}

type deliverStep struct {
	t module.DeliveryTarget

	// Resolver object used for obtaining information to include in Received
	// header.
	resolver Resolver
}

func LookupAddr(r Resolver, ip net.IP) (string, error) {
	names, err := r.LookupAddr(context.Background(), ip.String())
	if err != nil || len(names) == 0 {
		return "", err
	}
	return strings.TrimRight(names[0], "."), nil
}

func (step deliverStep) Pass(ctx *module.DeliveryContext, body io.Reader) (io.Reader, bool, error) {
	// We can safetly assume at least one recipient.
	to := sanitizeString(ctx.To[0])

	var received string
	if !ctx.DontTraceSender {
		received += "from " + ctx.SrcHostname
		if tcpAddr, ok := ctx.SrcAddr.(*net.TCPAddr); ok {
			domain, err := LookupAddr(step.resolver, tcpAddr.IP)
			if err != nil {
				received += fmt.Sprintf(" ([%v])", tcpAddr.IP)
			} else {
				received += fmt.Sprintf(" (%s [%v])", domain, tcpAddr.IP)
			}
		}
	}
	received += fmt.Sprintf(" by %s (envelope-sender <%s>)", sanitizeString(ctx.OurHostname), sanitizeString(ctx.From))
	received += fmt.Sprintf(" with %s id %s", ctx.SrcProto, ctx.DeliveryID)
	received += fmt.Sprintf(" for %s; %s", to, time.Now().Format(time.RFC1123Z))

	// Don't mutate passed context, since it will affect further steps.
	ctxCpy := ctx.DeepCopy()
	ctxCpy.Header.Add("Received", received)

	err := step.t.Deliver(*ctxCpy, body)
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
	case "rcpt_domain":
		for _, rcpt := range ctx.To {
			parts := strings.Split(rcpt, "@")
			if len(parts) != 2 {
				return nil, false, errors.New("invalid address")
			}
			if step.matches(parts[1]) {
				match = true
				break
			}
		}
	case "from":
		match = step.matches(ctx.From)
	case "from_domain":
		parts := strings.Split(ctx.From, "@")
		if len(parts) != 2 {
			return nil, false, errors.New("invalid address")
		}
		match = step.matches(parts[1])
	case "src_addr":
		match = step.matches(ctx.SrcAddr.String())
	case "src_hostname":
		match = step.matches(ctx.SrcHostname)
	}

	if !match {
		return nil, true, nil
	}

	return passThroughPipeline(step.substeps, ctx, msg)
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

type stopStep struct {
	errCode int
	errMsg  string
}

func (step stopStep) Pass(_ *module.DeliveryContext, _ io.Reader) (io.Reader, bool, error) {
	if step.errCode != 0 {
		return nil, false, &smtp.SMTPError{Code: step.errCode, Message: step.errMsg}
	}
	return nil, false, nil
}

func stopStepFromCfg(node config.Node) (SMTPPipelineStep, error) {
	switch len(node.Args) {
	case 0:
		return stopStep{}, nil
	case 2:
		var step stopStep
		var err error
		step.errCode, err = strconv.Atoi(node.Args[0])
		if err != nil {
			return nil, err
		}

		step.errMsg = node.Args[1]
		return step, nil
	default:
		return nil, config.NodeErr(&node, "expected 2 or 0 arguments")
	}
}

func filterStepFromCfg(globals map[string]interface{}, node *config.Node) (SMTPPipelineStep, error) {
	if len(node.Args) == 0 {
		return nil, config.NodeErr(node, "expected at least 1 argument")
	}

	modObj, err := moduleFromNode(node)
	if err != nil {
		return nil, config.NodeErr(node, "%s", err.Error())
	}

	filter, ok := modObj.(module.Filter)
	if !ok {
		return nil, config.NodeErr(node, "module %s doesn't implement filter interface", modObj.Name())
	}

	if node.Children != nil {
		if err := initInlineModule(modObj, globals, node); err != nil {
			return nil, err
		}
	}

	return filterStep{filter}, nil
}

func deliverStepFromCfg(globals map[string]interface{}, node *config.Node) (SMTPPipelineStep, error) {
	target, err := deliverTarget(globals, node)
	if err != nil {
		return nil, err
	}

	return deliverStep{t: target, resolver: net.DefaultResolver}, nil
}

func matchStepFromCfg(globals map[string]interface{}, node config.Node) (SMTPPipelineStep, error) {
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
		step, err := StepFromCfg(globals, child)
		if err != nil {
			return nil, err
		}
		res.substeps = append(res.substeps, step)
	}

	return res, nil
}

type destinationCond struct {
	value  string
	regexp *regexp.Regexp
}

func (cond destinationCond) matches(addr string) bool {
	if cond.regexp != nil {
		return cond.regexp.MatchString(addr)
	}

	if strings.Contains(cond.value, "@") {
		return cond.value == addr
	}

	parts := strings.Split(addr, "@")
	if len(parts) != 2 {
		return false
	}

	// Domains are always case-sensetive.
	return strings.ToLower(parts[1]) == strings.ToLower(cond.value)
}

type destinationStep struct {
	conds    []destinationCond
	substeps []SMTPPipelineStep
}

func (step *destinationStep) Pass(ctx *module.DeliveryContext, msg io.Reader) (io.Reader, bool, error) {
	ctxCpy := ctx.DeepCopy()
	unmatchedRcpts := make([]string, 0, len(ctx.To))
	ctxCpy.To = make([]string, 0, len(ctx.To))
	for _, rcpt := range ctx.To {
		matched := false
		for _, cond := range step.conds {
			if cond.matches(rcpt) {
				matched = true
			}
		}
		if matched {
			ctxCpy.To = append(ctxCpy.To, rcpt)
		} else {
			unmatchedRcpts = append(unmatchedRcpts, rcpt)
		}
	}

	if len(ctxCpy.To) == 0 {
		return nil, true, nil
	}

	// Remove matched recipients from original context value, so only one
	// 'destination' block will ever handle particular recipient.
	// Note: we are passing ctxCpy to substeps, not ctx.
	//		 ctxCpy.To contains matched rcpts, ctx.To contains unmatched
	ctx.To = unmatchedRcpts

	newBody, shouldContinue, err := passThroughPipeline(step.substeps, ctxCpy, msg)
	return newBody, shouldContinue && len(ctx.To) != 0, err
}

func seekableBody(body io.Reader) (io.ReadSeeker, error) {
	if seeker, ok := body.(io.ReadSeeker); ok {
		return seeker, nil
	}
	if byter, ok := body.(Byter); ok {
		return NewBytesReader(byter.Bytes()), nil
	}

	bodyBlob, err := ioutil.ReadAll(body)
	log.Debugln("message body copied or buffered, size:", len(bodyBlob))
	return NewBytesReader(bodyBlob), err
}

func passThroughPipeline(steps []SMTPPipelineStep, ctx *module.DeliveryContext, body io.Reader) (io.Reader, bool, error) {
	currentBody, err := seekableBody(body)
	if err != nil {
		return nil, false, err
	}

	for _, step := range steps {
		log.Debugf("running %T step...", step)
		r, cont, err := step.Pass(ctx, currentBody)
		if !cont {
			return nil, false, err
		}

		if r != nil {
			currentBody, err = seekableBody(r)
			if err != nil {
				return nil, false, err
			}
		}
		if _, err := currentBody.Seek(0, io.SeekStart); err != nil {
			return nil, false, err
		}
	}

	return body, true, nil
}

func destinationStepFromCfg(globals map[string]interface{}, node config.Node) (SMTPPipelineStep, error) {
	if len(node.Args) == 0 {
		return nil, errors.New("required at least one condition")
	}

	step := destinationStep{}

	for _, arg := range node.Args {
		if strings.HasPrefix(arg, "/") && strings.HasSuffix(arg, "/") {
			re, err := regexp.Compile(arg[1 : len(arg)-1])
			if err != nil {
				return nil, err
			}

			step.conds = append(step.conds, destinationCond{regexp: re})
		} else {
			step.conds = append(step.conds, destinationCond{value: arg})
		}
	}

	for _, child := range node.Children {
		substep, err := StepFromCfg(globals, child)
		if err != nil {
			return nil, err
		}
		step.substeps = append(step.substeps, substep)
	}

	return &step, nil
}

func StepFromCfg(globals map[string]interface{}, node config.Node) (SMTPPipelineStep, error) {
	switch node.Name {
	case "filter":
		return filterStepFromCfg(globals, &node)
	case "deliver":
		return deliverStepFromCfg(globals, &node)
	case "match":
		return matchStepFromCfg(globals, node)
	case "destination":
		return destinationStepFromCfg(globals, node)
	case "stop":
		return stopStepFromCfg(node)
	case "require_auth":
		return requireAuthStep{}, nil
	default:
		return nil, fmt.Errorf("unknown configuration directive or pipeline step: %s", node.Name)
	}
}
