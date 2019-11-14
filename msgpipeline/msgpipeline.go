package msgpipeline

import (
	"net"
	"strings"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/address"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/dns"
	"github.com/foxcpp/maddy/exterrors"
	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/modify"
	"github.com/foxcpp/maddy/module"
	"github.com/foxcpp/maddy/target"
)

// MsgPipeline is a object that is responsible for selecting delivery targets
// for the message and running necessary checks and modifiers.
//
// It implements module.DeliveryTarget.
//
// It is not a "module object" and is intended to be used as part of message
// source (Submission, SMTP, JMAP modules) implementation.
type MsgPipeline struct {
	msgpipelineCfg
	Hostname string
	Resolver dns.Resolver

	Log log.Logger
}

type sourceBlock struct {
	checks      []module.Check
	modifiers   modify.Group
	rejectErr   error
	perRcpt     map[string]*rcptBlock
	defaultRcpt *rcptBlock
}

type rcptBlock struct {
	checks    []module.Check
	modifiers modify.Group
	rejectErr error
	targets   []module.DeliveryTarget
}

func New(globals map[string]interface{}, cfg []config.Node) (*MsgPipeline, error) {
	parsedCfg, err := parseMsgPipelineRootCfg(globals, cfg)
	return &MsgPipeline{
		msgpipelineCfg: parsedCfg,
		Resolver:       net.DefaultResolver,
	}, err
}

func (d *MsgPipeline) RunEarlyChecks(state *smtp.ConnectionState) error {
	// TODO: See if there is some point in parallelization of this
	// function.
	for _, check := range d.globalChecks {
		earlyCheck, ok := check.(module.EarlyCheck)
		if !ok {
			continue
		}

		if err := earlyCheck.CheckConnection(state); err != nil {
			return err
		}
	}
	return nil
}

// Start starts new message delivery, runs connection and sender checks, sender modifiers
// and selects source block from config to use for handling.
//
// Returned module.Delivery implements PartialDelivery. If underlying target doesn't
// support it, msgpipeline will copy the returned error for all recipients handled
// by target.
func (d *MsgPipeline) Start(msgMeta *module.MsgMetadata, mailFrom string) (module.Delivery, error) {
	dd := msgpipelineDelivery{
		d:                  d,
		rcptModifiersState: make(map[*rcptBlock]module.ModifierState),
		deliveries:         make(map[module.DeliveryTarget]*delivery),
		msgMeta:            msgMeta,
		log:                target.DeliveryLogger(d.Log, msgMeta),
	}
	dd.checkRunner = newCheckRunner(msgMeta, dd.log)
	dd.checkRunner.doDMARC = d.doDMARC
	dd.checkRunner.resolver = d.Resolver

	if msgMeta.OriginalRcpts == nil {
		msgMeta.OriginalRcpts = map[string]string{}
	}

	if err := dd.start(msgMeta, mailFrom); err != nil {
		dd.close()
		return nil, err
	}

	return &dd, nil
}

func (dd *msgpipelineDelivery) start(msgMeta *module.MsgMetadata, mailFrom string) error {
	var err error

	if err := dd.checkRunner.checkConnSender(dd.d.globalChecks, mailFrom); err != nil {
		return err
	}

	if mailFrom, err = dd.initRunGlobalModifiers(msgMeta, mailFrom); err != nil {
		return err
	}

	sourceBlock, err := dd.srcBlockForAddr(mailFrom)
	if err != nil {
		return err
	}
	if sourceBlock.rejectErr != nil {
		dd.log.Debugf("sender %s rejected with error: %v", mailFrom, sourceBlock.rejectErr)
		return sourceBlock.rejectErr
	}
	dd.sourceBlock = sourceBlock

	if err := dd.checkRunner.checkConnSender(sourceBlock.checks, mailFrom); err != nil {
		return err
	}

	sourceModifiersState, err := sourceBlock.modifiers.ModStateForMsg(msgMeta)
	if err != nil {
		return err
	}
	mailFrom, err = sourceModifiersState.RewriteSender(mailFrom)
	if err != nil {
		return err
	}
	dd.sourceModifiersState = sourceModifiersState

	dd.sourceAddr = mailFrom
	return nil
}

func (dd *msgpipelineDelivery) initRunGlobalModifiers(msgMeta *module.MsgMetadata, mailFrom string) (string, error) {
	globalModifiersState, err := dd.d.globalModifiers.ModStateForMsg(msgMeta)
	if err != nil {
		return "", err
	}
	mailFrom, err = globalModifiersState.RewriteSender(mailFrom)
	if err != nil {
		globalModifiersState.Close()
		return "", err
	}
	dd.globalModifiersState = globalModifiersState
	return mailFrom, nil
}

func (dd *msgpipelineDelivery) srcBlockForAddr(mailFrom string) (sourceBlock, error) {
	// First try to match against complete address.
	srcBlock, ok := dd.d.perSource[strings.ToLower(mailFrom)]
	if !ok {
		// Then try domain-only.
		_, domain, err := address.Split(mailFrom)
		// mailFrom != "" is added as a special condition
		// instead of extending address.Split because ""
		// is not a valid RFC 282 address and only a special
		// value for SMTP.
		if err != nil && mailFrom != "" {
			return sourceBlock{}, &exterrors.SMTPError{
				Code:         501,
				EnhancedCode: exterrors.EnhancedCode{5, 1, 3},
				Message:      "Invalid sender address",
				Err:          err,
				Misc: map[string]interface{}{
					"reason": "Can't extract local-part and host-part",
				},
			}
		}

		srcBlock, ok = dd.d.perSource[strings.ToLower(domain)]
		if !ok {
			// Fallback to the default source block.
			srcBlock = dd.d.defaultSource
			dd.log.Debugf("sender %s matched by default rule", mailFrom)
		} else {
			dd.log.Debugf("sender %s matched by domain rule '%s'", mailFrom, strings.ToLower(domain))
		}
	} else {
		dd.log.Debugf("sender %s matched by address rule '%s'", mailFrom, strings.ToLower(mailFrom))
	}
	return srcBlock, nil
}

type delivery struct {
	module.Delivery
	// Recipient addresses this delivery object is used for, original values (not modified by RewriteRcpt).
	recipients []string
}

type msgpipelineDelivery struct {
	d *MsgPipeline

	globalModifiersState module.ModifierState
	sourceModifiersState module.ModifierState
	rcptModifiersState   map[*rcptBlock]module.ModifierState

	log log.Logger

	sourceAddr  string
	sourceBlock sourceBlock

	deliveries  map[module.DeliveryTarget]*delivery
	msgMeta     *module.MsgMetadata
	checkRunner *checkRunner
}

func (dd *msgpipelineDelivery) AddRcpt(to string) error {
	if err := dd.checkRunner.checkRcpt(dd.d.globalChecks, to); err != nil {
		return err
	}
	if err := dd.checkRunner.checkRcpt(dd.sourceBlock.checks, to); err != nil {
		return err
	}

	originalTo := to

	newTo, err := dd.globalModifiersState.RewriteRcpt(to)
	if err != nil {
		return err
	}
	dd.log.Debugln("global rcpt modifiers:", to, "=>", newTo)
	to = newTo
	newTo, err = dd.sourceModifiersState.RewriteRcpt(to)
	if err != nil {
		return err
	}
	dd.log.Debugln("per-source rcpt modifiers:", to, "=>", newTo)
	to = newTo

	rcptBlock, err := dd.rcptBlockForAddr(to)
	if err != nil {
		return err
	}

	if rcptBlock.rejectErr != nil {
		return rcptBlock.rejectErr
	}

	if err := dd.checkRunner.checkRcpt(rcptBlock.checks, to); err != nil {
		return err
	}

	rcptModifiersState, err := dd.getRcptModifiers(rcptBlock, to)
	if err != nil {
		return err
	}

	newTo, err = rcptModifiersState.RewriteRcpt(to)
	if err != nil {
		rcptModifiersState.Close()
		return err
	}
	dd.log.Debugln("per-rcpt modifiers:", to, "=>", newTo)
	to = newTo

	if originalTo != to {
		dd.msgMeta.OriginalRcpts[to] = originalTo
	}

	for _, tgt := range rcptBlock.targets {
		delivery, err := dd.getDelivery(tgt)
		if err != nil {
			return err
		}

		if err := delivery.AddRcpt(to); err != nil {
			return err
		}
		delivery.recipients = append(delivery.recipients, originalTo)
	}

	return nil
}

func (dd *msgpipelineDelivery) Body(header textproto.Header, body buffer.Buffer) error {
	if err := dd.checkRunner.checkBody(dd.d.globalChecks, header, body); err != nil {
		return err
	}
	if err := dd.checkRunner.checkBody(dd.sourceBlock.checks, header, body); err != nil {
		return err
	}
	// TODO: Decide whether per-recipient body checks should be executed.

	if err := dd.checkRunner.applyResults(dd.d.Hostname, &header); err != nil {
		return err
	}

	// Run modifiers after Authentication-Results addition to make
	// sure signatures, etc will cover it.
	if err := dd.globalModifiersState.RewriteBody(header, body); err != nil {
		return err
	}
	if err := dd.sourceModifiersState.RewriteBody(header, body); err != nil {
		return err
	}

	for _, delivery := range dd.deliveries {
		if err := delivery.Body(header, body); err != nil {
			return err
		}
		dd.log.Debugf("delivery.Body ok, Delivery object = %T", delivery)
	}
	return nil
}

// statusCollector wraps StatusCollector and adds reverse translation
// of recipients for all statuses.]
//
// We can't let delivery targets set statuses directly because they see
// modified addresses (RewriteRcpt) and we are supposed to report
// statuses using original values. Additionally, we should still avoid
// collect-and-them-report approach since statuses should be reported
// as soon as possible (that is required by LMTP).
type statusCollector struct {
	originalRcpts map[string]string
	wrapped       module.StatusCollector
}

func (sc statusCollector) SetStatus(rcptTo string, err error) {
	original, ok := sc.originalRcpts[rcptTo]
	if ok {
		rcptTo = original
	}
	sc.wrapped.SetStatus(rcptTo, err)
}

func (dd *msgpipelineDelivery) BodyNonAtomic(c module.StatusCollector, header textproto.Header, body buffer.Buffer) {
	setStatusAll := func(err error) {
		for _, delivery := range dd.deliveries {
			for _, rcpt := range delivery.recipients {
				c.SetStatus(rcpt, err)
			}
		}
	}

	if err := dd.checkRunner.checkBody(dd.d.globalChecks, header, body); err != nil {
		setStatusAll(err)
		return
	}
	if err := dd.checkRunner.checkBody(dd.sourceBlock.checks, header, body); err != nil {
		setStatusAll(err)
		return
	}

	// Run modifiers after Authentication-Results addition to make
	// sure signatures, etc will cover it.
	if err := dd.globalModifiersState.RewriteBody(header, body); err != nil {
		setStatusAll(err)
		return
	}
	if err := dd.sourceModifiersState.RewriteBody(header, body); err != nil {
		setStatusAll(err)
		return
	}

	for _, delivery := range dd.deliveries {
		partDelivery, ok := delivery.Delivery.(module.PartialDelivery)
		if ok {
			partDelivery.BodyNonAtomic(statusCollector{
				originalRcpts: dd.msgMeta.OriginalRcpts,
				wrapped:       c,
			}, header, body)
			continue
		}

		if err := delivery.Body(header, body); err != nil {
			for _, rcpt := range delivery.recipients {
				c.SetStatus(rcpt, err)
			}
		}
	}
}

func (dd msgpipelineDelivery) Commit() error {
	dd.close()

	for _, delivery := range dd.deliveries {
		if err := delivery.Commit(); err != nil {
			// No point in Committing remaining deliveries, everything is broken already.
			return err
		}
	}
	return nil
}

func (dd *msgpipelineDelivery) close() {
	dd.checkRunner.close()

	if dd.globalModifiersState != nil {
		dd.globalModifiersState.Close()
	}
	if dd.sourceModifiersState != nil {
		dd.sourceModifiersState.Close()
	}
	for _, modifiers := range dd.rcptModifiersState {
		modifiers.Close()
	}
}

func (dd msgpipelineDelivery) Abort() error {
	dd.close()

	var lastErr error
	for _, delivery := range dd.deliveries {
		if err := delivery.Abort(); err != nil {
			dd.log.Debugf("delivery.Abort failure, Delivery object = %T: %v", delivery, err)
			lastErr = err
			// Continue anyway and try to Abort all remaining delivery objects.
		}
	}
	return lastErr
}

func (dd *msgpipelineDelivery) rcptBlockForAddr(rcptTo string) (*rcptBlock, error) {
	// First try to match against complete address.
	rcptBlock, ok := dd.sourceBlock.perRcpt[strings.ToLower(rcptTo)]
	if !ok {
		// Then try domain-only.
		_, domain, err := address.Split(rcptTo)
		if err != nil {
			return nil, &exterrors.SMTPError{
				Code:         501,
				EnhancedCode: exterrors.EnhancedCode{5, 1, 3},
				Message:      "Invalid recipient address",
				Err:          err,
				Misc: map[string]interface{}{
					"reason": "Can't extract local-part and host-part",
				},
			}
		}

		rcptBlock, ok = dd.sourceBlock.perRcpt[strings.ToLower(domain)]
		if !ok {
			// Fallback to the default source block.
			rcptBlock = dd.sourceBlock.defaultRcpt
			dd.log.Debugf("recipient %s matched by default rule", rcptTo)
		} else {
			dd.log.Debugf("recipient %s matched by domain rule '%s'", rcptTo, strings.ToLower(domain))
		}
	} else {
		dd.log.Debugf("recipient %s matched by address rule '%s'", rcptTo, strings.ToLower(rcptTo))
	}
	return rcptBlock, nil
}

func (dd *msgpipelineDelivery) getRcptModifiers(rcptBlock *rcptBlock, rcptTo string) (module.ModifierState, error) {
	rcptModifiersState, ok := dd.rcptModifiersState[rcptBlock]
	if ok {
		return rcptModifiersState, nil
	}

	rcptModifiersState, err := rcptBlock.modifiers.ModStateForMsg(dd.msgMeta)
	if err != nil {
		return nil, err
	}

	newSender, err := rcptModifiersState.RewriteSender(dd.sourceAddr)
	if err == nil && newSender != dd.sourceAddr {
		dd.log.Msg("Per-recipient modifier changed sender address. This is not supported and will "+
			"be ignored.", "rcpt", rcptTo, "originalFrom", dd.sourceAddr, "modifiedFrom", newSender)
	}

	dd.rcptModifiersState[rcptBlock] = rcptModifiersState
	return rcptModifiersState, nil
}

func (dd *msgpipelineDelivery) getDelivery(tgt module.DeliveryTarget) (*delivery, error) {
	delivery_, ok := dd.deliveries[tgt]
	if ok {
		return delivery_, nil
	}

	deliveryObj, err := tgt.Start(dd.msgMeta, dd.sourceAddr)
	if err != nil {
		dd.log.Debugf("tgt.Start(%s) failure, target = %s: %v", dd.sourceAddr, objectName(tgt), err)
		return nil, err
	}
	delivery_ = &delivery{Delivery: deliveryObj}

	dd.log.Debugf("tgt.Start(%s) ok, target = %s", dd.sourceAddr, objectName(tgt))

	dd.deliveries[tgt] = delivery_
	return delivery_, nil
}
