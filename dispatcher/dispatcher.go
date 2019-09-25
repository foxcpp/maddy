package dispatcher

import (
	"context"
	"fmt"
	"strings"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-msgauth/authres"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/address"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/check"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/modify"
	"github.com/foxcpp/maddy/module"
	"github.com/foxcpp/maddy/target"
)

// Dispatcher is a object that is responsible for selecting delivery targets
// for the message and running necessary checks and modifier.
//
// It implements module.DeliveryTarget.
//
// It is not a "module object" and is intended to be used as part of message
// source (Submission, SMTP, JMAP modules) implementation.
type Dispatcher struct {
	dispatcherCfg
	Hostname string

	Log log.Logger
}

type sourceBlock struct {
	checks      check.Group
	modifiers   modify.Group
	rejectErr   error
	perRcpt     map[string]*rcptBlock
	defaultRcpt *rcptBlock
}

type rcptBlock struct {
	checks    check.Group
	modifiers modify.Group
	rejectErr error
	targets   []module.DeliveryTarget
}

func NewDispatcher(globals map[string]interface{}, cfg []config.Node) (*Dispatcher, error) {
	parsedCfg, err := parseDispatcherRootCfg(globals, cfg)
	return &Dispatcher{
		dispatcherCfg: parsedCfg,
	}, err
}

// Start starts new message delivery, runs connection and sender checks, sender modifiers
// and selects source block from config to use for handling.
//
// Returned module.Delivery implements PartialDelivery. If underlying target doesn't
// support it, dispatcher will copy the returned error for all recipients handled
// by target.
func (d *Dispatcher) Start(msgMeta *module.MsgMetadata, mailFrom string) (module.Delivery, error) {
	dd := dispatcherDelivery{
		d:                  d,
		rcptChecksState:    make(map[*rcptBlock]module.CheckState),
		rcptModifiersState: make(map[*rcptBlock]module.ModifierState),
		deliveries:         make(map[module.DeliveryTarget]*delivery),
		msgMeta:            msgMeta,
		log:                target.DeliveryLogger(d.Log, msgMeta),
		checkScore:         0,
	}

	if msgMeta.OriginalRcpts == nil {
		msgMeta.OriginalRcpts = map[string]string{}
	}

	if err := dd.start(msgMeta, mailFrom); err != nil {
		dd.close()
		return nil, err
	}

	return &dd, nil
}

func (dd *dispatcherDelivery) start(msgMeta *module.MsgMetadata, mailFrom string) error {
	var err error

	if err := dd.initRunGlobalChecks(msgMeta, mailFrom); err != nil {
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

	if err := dd.initRunSourceChecks(sourceBlock, msgMeta, mailFrom); err != nil {
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

func (dd *dispatcherDelivery) initRunGlobalChecks(msgMeta *module.MsgMetadata, mailFrom string) error {
	globalChecksState, err := dd.d.globalChecks.CheckStateForMsg(msgMeta)
	if err != nil {
		return err
	}
	if err := dd.checkResult(globalChecksState.CheckConnection(context.TODO())); err != nil {
		globalChecksState.Close()
		return err
	}
	if err := dd.checkResult(globalChecksState.CheckSender(context.TODO(), mailFrom)); err != nil {
		globalChecksState.Close()
		return err
	}
	dd.globalChecksState = globalChecksState
	return nil
}

func (dd *dispatcherDelivery) initRunGlobalModifiers(msgMeta *module.MsgMetadata, mailFrom string) (string, error) {
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

func (dd *dispatcherDelivery) initRunSourceChecks(srcBlock sourceBlock, msgMeta *module.MsgMetadata, mailFrom string) error {
	sourceChecksState, err := srcBlock.checks.CheckStateForMsg(msgMeta)
	if err != nil {
		return err
	}
	if err := dd.checkResult(sourceChecksState.CheckConnection(context.TODO())); err != nil {
		sourceChecksState.Close()
		return err
	}
	if err := dd.checkResult(sourceChecksState.CheckSender(context.TODO(), mailFrom)); err != nil {
		sourceChecksState.Close()
		return err
	}
	dd.sourceChecksState = sourceChecksState
	return nil
}

func (dd *dispatcherDelivery) srcBlockForAddr(mailFrom string) (sourceBlock, error) {
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
			return sourceBlock{}, &smtp.SMTPError{
				Code:         501,
				EnhancedCode: smtp.EnhancedCode{5, 1, 3},
				Message:      "Invalid sender address: " + err.Error(),
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

type dispatcherDelivery struct {
	d *Dispatcher

	globalChecksState module.CheckState
	sourceChecksState module.CheckState
	rcptChecksState   map[*rcptBlock]module.CheckState

	globalModifiersState module.ModifierState
	sourceModifiersState module.ModifierState
	rcptModifiersState   map[*rcptBlock]module.ModifierState

	log log.Logger

	sourceAddr  string
	sourceBlock sourceBlock

	deliveries map[module.DeliveryTarget]*delivery
	msgMeta    *module.MsgMetadata
	cancelCtx  context.Context
	checkScore int32
	authRes    []authres.Result
	header     textproto.Header
}

func (dd *dispatcherDelivery) AddRcpt(to string) error {
	if err := dd.checkResult(dd.globalChecksState.CheckRcpt(context.TODO(), to)); err != nil {
		return err
	}
	if err := dd.checkResult(dd.sourceChecksState.CheckRcpt(context.TODO(), to)); err != nil {
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
		dd.log.Debugf("recipient %s rejected: %v", to, rcptBlock.rejectErr)
		return rcptBlock.rejectErr
	}

	rcptChecksState, err := dd.getRcptChecks(rcptBlock)
	if err != nil {
		return err
	}
	if err := dd.checkResult(rcptChecksState.CheckRcpt(context.TODO(), to)); err != nil {
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
			dd.log.Debugf("delivery.AddRcpt(%s) failure, Delivery object = %T: %v", to, delivery, err)
			return err
		}
		dd.log.Debugf("delivery.AddRcpt(%s) ok, Delivery object = %T", to, delivery)
		delivery.recipients = append(delivery.recipients, originalTo)
	}

	return nil
}

func (dd *dispatcherDelivery) Body(header textproto.Header, body buffer.Buffer) error {
	if err := dd.runBodyChecks(&header, body); err != nil {
		return err
	}

	// After results for all checks are checked, authRes will be populated with values
	// we should put into Authentication-Results header.
	if len(dd.authRes) != 0 {
		header.Add("Authentication-Results", authres.Format(dd.d.Hostname, dd.authRes))
	}
	for field := dd.header.Fields(); field.Next(); {
		header.Add(field.Key(), field.Value())
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
			dd.log.Debugf("delivery.Body failure, Delivery object = %T: %v", delivery, err)
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

func (dd *dispatcherDelivery) BodyNonAtomic(c module.StatusCollector, header textproto.Header, body buffer.Buffer) {
	setStatusAll := func(err error) {
		for _, delivery := range dd.deliveries {
			for _, rcpt := range delivery.recipients {
				c.SetStatus(rcpt, err)
			}
		}
	}

	if err := dd.runBodyChecks(&header, body); err != nil {
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
			dd.log.Debugf("delivery.Body failure, Delivery object = %T: %v", delivery, err)
			for _, rcpt := range delivery.recipients {
				c.SetStatus(rcpt, err)
			}
		}
		dd.log.Debugf("delivery.Body ok, Delivery object = %T", delivery)
	}
}

func (dd dispatcherDelivery) Commit() error {
	dd.close()

	for _, delivery := range dd.deliveries {
		if err := delivery.Commit(); err != nil {
			dd.log.Debugf("delivery.Commit failure, Delivery object = %T: %v", delivery, err)
			// No point in Committing remaining deliveries, everything is broken already.
			return err
		}
		dd.log.Debugf("delivery.Commit ok, Delivery object = %T", delivery)
	}
	return nil
}

func (dd *dispatcherDelivery) close() {
	// these fields can be nil if delivery is partially initialized (in Dispatcher.Start method)
	if dd.globalChecksState != nil {
		dd.globalChecksState.Close()
	}
	if dd.sourceChecksState != nil {
		dd.sourceChecksState.Close()
	}
	for _, check_ := range dd.rcptChecksState {
		check_.Close()
	}

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

func (dd dispatcherDelivery) Abort() error {
	dd.close()

	var lastErr error
	for _, delivery := range dd.deliveries {
		if err := delivery.Abort(); err != nil {
			dd.log.Debugf("delivery.Abort failure, Delivery object = %T: %v", delivery, err)
			lastErr = err
			// Continue anyway and try to Abort all remaining delivery objects.
		}
		dd.log.Debugf("delivery.Abort ok, Delivery object = %T", delivery)
	}
	dd.log.Debugf("delivery aborted")
	return lastErr
}

func (dd *dispatcherDelivery) checkResult(checkResult module.CheckResult) error {
	if checkResult.RejectErr != nil {
		return checkResult.RejectErr
	}
	if checkResult.Quarantine {
		dd.log.Printf("quarantined message due to check result")
		dd.msgMeta.Quarantine = true
	}
	dd.checkScore += checkResult.ScoreAdjust
	if dd.d.rejectScore != nil && dd.checkScore >= int32(*dd.d.rejectScore) {
		dd.log.Debugf("score %d >= %d, rejecting", dd.checkScore, *dd.d.rejectScore)
		return &smtp.SMTPError{
			Code:         550,
			EnhancedCode: smtp.EnhancedCode{5, 7, 0},
			Message:      fmt.Sprintf("Message is rejected due to multiple local policy violations (score %d)", dd.checkScore),
		}
	}
	if dd.d.quarantineScore != nil && dd.checkScore >= int32(*dd.d.quarantineScore) {
		if !dd.msgMeta.Quarantine {
			dd.log.Printf("quarantined message due to score %d >= %d", dd.checkScore, *dd.d.quarantineScore)
		}
		dd.msgMeta.Quarantine = true
	}
	if len(checkResult.AuthResult) != 0 {
		dd.authRes = append(dd.authRes, checkResult.AuthResult...)
	}
	for field := checkResult.Header.Fields(); field.Next(); {
		dd.header.Add(field.Key(), field.Value())
	}
	return nil
}

func (dd *dispatcherDelivery) rcptBlockForAddr(rcptTo string) (*rcptBlock, error) {
	// First try to match against complete address.
	rcptBlock, ok := dd.sourceBlock.perRcpt[strings.ToLower(rcptTo)]
	if !ok {
		// Then try domain-only.
		_, domain, err := address.Split(rcptTo)
		if err != nil {
			return nil, &smtp.SMTPError{
				Code:         501,
				EnhancedCode: smtp.EnhancedCode{5, 1, 3},
				Message:      "Invalid recipient address: " + err.Error(),
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

func (dd *dispatcherDelivery) getRcptChecks(rcptBlock *rcptBlock) (module.CheckState, error) {
	rcptChecksState, ok := dd.rcptChecksState[rcptBlock]
	if ok {
		return rcptChecksState, nil
	}

	rcptChecksState, err := rcptBlock.checks.CheckStateForMsg(dd.msgMeta)
	if err != nil {
		return nil, err
	}

	if err := dd.checkResult(rcptChecksState.CheckConnection(context.TODO())); err != nil {
		rcptChecksState.Close()
		return nil, err
	}
	if err := dd.checkResult(rcptChecksState.CheckSender(context.TODO(), dd.sourceAddr)); err != nil {
		rcptChecksState.Close()
		return nil, err
	}
	dd.rcptChecksState[rcptBlock] = rcptChecksState
	return rcptChecksState, nil
}

func (dd *dispatcherDelivery) getRcptModifiers(rcptBlock *rcptBlock, rcptTo string) (module.ModifierState, error) {
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
		dd.log.Printf("Per-recipient modifier changed sender address. This is not supported and will "+
			"be ignored. RCPT TO = %s, original MAIL FROM = %s, modified MAIL FROM = %s",
			rcptTo, dd.sourceAddr, newSender)
	}

	dd.rcptModifiersState[rcptBlock] = rcptModifiersState
	return rcptModifiersState, nil
}

func (dd *dispatcherDelivery) getDelivery(tgt module.DeliveryTarget) (*delivery, error) {
	delivery_, ok := dd.deliveries[tgt]
	if ok {
		return delivery_, nil
	}

	deliveryObj, err := tgt.Start(dd.msgMeta, dd.sourceAddr)
	if err != nil {
		dd.log.Debugf("tgt.Start(%s) failure, target = %s (%s): %v",
			dd.sourceAddr, tgt.(module.Module).InstanceName(), tgt.(module.Module).Name(), err)
		return nil, err
	}
	delivery_ = &delivery{Delivery: deliveryObj}

	dd.log.Debugf("tgt.Start(%s) ok, target = %s (%s)",
		dd.sourceAddr, tgt.(module.Module).InstanceName(), tgt.(module.Module).Name())

	dd.deliveries[tgt] = delivery_
	return delivery_, nil
}

func (dd *dispatcherDelivery) runBodyChecks(header *textproto.Header, body buffer.Buffer) error {
	if err := dd.checkResult(dd.globalChecksState.CheckBody(context.TODO(), *header, body)); err != nil {
		return err
	}
	if err := dd.checkResult(dd.sourceChecksState.CheckBody(context.TODO(), *header, body)); err != nil {
		return err
	}
	for _, rcptChecksState := range dd.rcptChecksState {
		if err := dd.checkResult(rcptChecksState.CheckBody(context.TODO(), *header, body)); err != nil {
			return err
		}
	}

	// After results for all checks are checked, authRes will be populated with values
	// we should put into Authentication-Results header.
	if len(dd.authRes) != 0 {
		header.Add("Authentication-Results", authres.Format(dd.d.Hostname, dd.authRes))
	}
	for field := dd.header.Fields(); field.Next(); {
		header.Add(field.Key(), field.Value())
	}

	return nil
}
