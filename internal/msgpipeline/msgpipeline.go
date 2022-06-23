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
	"context"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/framework/address"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/dns"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/modify"
	"github.com/foxcpp/maddy/internal/target"
	"golang.org/x/sync/errgroup"
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

	// Used to indicate the pipeline is handling messages received from the
	// external source and not from any other module. That is, this MsgPipeline
	// is an instance embedded in endpoint/smtp implementation, for example.
	//
	// This is a hack since only MsgPipeline can execute some operations at the
	// right time but it is not a good idea to execute them multiple multiple
	// times for a single message that might be actually handled my multiple
	// pipelines via 'msgpipeline' module or 'reroute' directive.
	//
	// At the moment, the only such operation is the addition of the Received
	// header field. See where it happens for explanation on why it is done
	// exactly in this place.
	FirstPipeline bool

	Log log.Logger
}

type rcptIn struct {
	t     module.Table
	block *rcptBlock
}

type sourceBlock struct {
	checks      []module.Check
	modifiers   modify.Group
	rejectErr   error
	rcptIn      []rcptIn
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
		Resolver:       dns.DefaultResolver(),
	}, err
}

func (d *MsgPipeline) RunEarlyChecks(ctx context.Context, state *smtp.ConnectionState) error {
	eg, checkCtx := errgroup.WithContext(ctx)

	// TODO: See if there is some point in parallelization of this
	// function.
	for _, check := range d.globalChecks {
		earlyCheck, ok := check.(module.EarlyCheck)
		if !ok {
			continue
		}

		eg.Go(func() error {
			return earlyCheck.CheckConnection(checkCtx, state)
		})
	}
	return eg.Wait()
}

// Start starts new message delivery, runs connection and sender checks, sender modifiers
// and selects source block from config to use for handling.
//
// Returned module.Delivery implements PartialDelivery. If underlying target doesn't
// support it, msgpipeline will copy the returned error for all recipients handled
// by target.
func (d *MsgPipeline) Start(ctx context.Context, msgMeta *module.MsgMetadata, mailFrom string) (module.Delivery, error) {
	dd := msgpipelineDelivery{
		d:                  d,
		rcptModifiersState: make(map[*rcptBlock]module.ModifierState),
		deliveries:         make(map[module.DeliveryTarget]*delivery),
		msgMeta:            msgMeta,
		log:                target.DeliveryLogger(d.Log, msgMeta),
	}
	dd.checkRunner = newCheckRunner(msgMeta, dd.log, d.Resolver)
	dd.checkRunner.doDMARC = d.doDMARC

	if msgMeta.OriginalRcpts == nil {
		msgMeta.OriginalRcpts = map[string]string{}
	}

	if err := dd.start(ctx, msgMeta, mailFrom); err != nil {
		dd.close()
		return nil, err
	}

	return &dd, nil
}

func (dd *msgpipelineDelivery) start(ctx context.Context, msgMeta *module.MsgMetadata, mailFrom string) error {
	var err error

	if err := dd.checkRunner.checkConnSender(ctx, dd.d.globalChecks, mailFrom); err != nil {
		return err
	}

	if mailFrom, err = dd.initRunGlobalModifiers(ctx, msgMeta, mailFrom); err != nil {
		return err
	}

	sourceBlock, err := dd.srcBlockForAddr(ctx, mailFrom)
	if err != nil {
		return err
	}
	if sourceBlock.rejectErr != nil {
		dd.log.Debugf("sender %s rejected with error: %v", mailFrom, sourceBlock.rejectErr)
		return sourceBlock.rejectErr
	}
	dd.sourceBlock = sourceBlock

	if err := dd.checkRunner.checkConnSender(ctx, sourceBlock.checks, mailFrom); err != nil {
		return err
	}

	sourceModifiersState, err := sourceBlock.modifiers.ModStateForMsg(ctx, msgMeta)
	if err != nil {
		return err
	}
	mailFrom, err = sourceModifiersState.RewriteSender(ctx, mailFrom)
	if err != nil {
		return err
	}
	dd.sourceModifiersState = sourceModifiersState

	dd.sourceAddr = mailFrom
	return nil
}

func (dd *msgpipelineDelivery) initRunGlobalModifiers(ctx context.Context, msgMeta *module.MsgMetadata, mailFrom string) (string, error) {
	globalModifiersState, err := dd.d.globalModifiers.ModStateForMsg(ctx, msgMeta)
	if err != nil {
		return "", err
	}
	mailFrom, err = globalModifiersState.RewriteSender(ctx, mailFrom)
	if err != nil {
		globalModifiersState.Close()
		return "", err
	}
	dd.globalModifiersState = globalModifiersState
	return mailFrom, nil
}

func (dd *msgpipelineDelivery) srcBlockForAddr(ctx context.Context, mailFrom string) (sourceBlock, error) {
	cleanFrom := mailFrom
	if mailFrom != "" {
		var err error
		cleanFrom, err = address.ForLookup(mailFrom)
		if err != nil {
			return sourceBlock{}, &exterrors.SMTPError{
				Code:         501,
				EnhancedCode: exterrors.EnhancedCode{5, 1, 7},
				Message:      "Unable to normalize the sender address",
				Err:          err,
			}
		}
	}

	for _, srcIn := range dd.d.sourceIn {
		_, ok, err := srcIn.t.Lookup(ctx, cleanFrom)
		if err != nil {
			dd.log.Error("source_in lookup failed", err, "key", cleanFrom)
			continue
		}
		if !ok {
			continue
		}
		return srcIn.block, nil
	}

	// First try to match against complete address.
	srcBlock, ok := dd.d.perSource[cleanFrom]
	if !ok {
		// Then try domain-only.
		_, domain, err := address.Split(cleanFrom)
		// mailFrom != "" is added as a special condition
		// instead of extending address.Split because ""
		// is not a valid RFC 282 address and only a special
		// value for SMTP.
		if err != nil && cleanFrom != "" {
			return sourceBlock{}, &exterrors.SMTPError{
				Code:         501,
				EnhancedCode: exterrors.EnhancedCode{5, 1, 3},
				Message:      "Invalid sender address",
				Err:          err,
				Reason:       "Can't extract local-part and host-part",
			}
		}

		// domain is already case-folded and normalized by the message source.
		srcBlock, ok = dd.d.perSource[domain]
		if !ok {
			// Fallback to the default source block.
			srcBlock = dd.d.defaultSource
			dd.log.Debugf("sender %s matched by default rule", mailFrom)
		} else {
			dd.log.Debugf("sender %s matched by domain rule '%s'", mailFrom, domain)
		}
	} else {
		dd.log.Debugf("sender %s matched by address rule '%s'", mailFrom, cleanFrom)
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

func (dd *msgpipelineDelivery) AddRcpt(ctx context.Context, to string) error {
	if err := dd.checkRunner.checkRcpt(ctx, dd.d.globalChecks, to); err != nil {
		return err
	}
	if err := dd.checkRunner.checkRcpt(ctx, dd.sourceBlock.checks, to); err != nil {
		return err
	}

	originalTo := to

	newTo, err := dd.globalModifiersState.RewriteRcpt(ctx, to)
	if err != nil {
		return err
	}
	dd.log.Debugln("global rcpt modifiers:", to, "=>", newTo)
	resultTo := newTo
	newTo = []string{}

	for _, to = range resultTo {
		var tempTo []string
		tempTo, err = dd.sourceModifiersState.RewriteRcpt(ctx, to)
		if err != nil {
			return err
		}
		newTo = append(newTo, tempTo...)
	}
	dd.log.Debugln("per-source rcpt modifiers:", to, "=>", newTo)
	resultTo = newTo

	for _, to = range resultTo {
		wrapErr := func(err error) error {
			return exterrors.WithFields(err, map[string]interface{}{
				"effective_rcpt": to,
			})
		}

		rcptBlock, err := dd.rcptBlockForAddr(ctx, to)
		if err != nil {
			return wrapErr(err)
		}

		if rcptBlock.rejectErr != nil {
			return wrapErr(rcptBlock.rejectErr)
		}

		if err := dd.checkRunner.checkRcpt(ctx, rcptBlock.checks, to); err != nil {
			return wrapErr(err)
		}

		rcptModifiersState, err := dd.getRcptModifiers(ctx, rcptBlock, to)
		if err != nil {
			return wrapErr(err)
		}

		newTo, err = rcptModifiersState.RewriteRcpt(ctx, to)
		if err != nil {
			rcptModifiersState.Close()
			return wrapErr(err)
		}
		dd.log.Debugln("per-rcpt modifiers:", to, "=>", newTo)

		for _, to = range newTo {
			wrapErr = func(err error) error {
				return exterrors.WithFields(err, map[string]interface{}{
					"effective_rcpt": to,
				})
			}

			if originalTo != to {
				dd.msgMeta.OriginalRcpts[to] = originalTo
			}

			for _, tgt := range rcptBlock.targets {
				// Do not wrap errors coming from nested pipeline target delivery since
				// that pipeline itself will insert effective_rcpt field and could do
				// its own rewriting - we do not want to hide it from the admin in
				// error messages.
				wrapErr := wrapErr
				if _, ok := tgt.(*MsgPipeline); ok {
					wrapErr = func(err error) error { return err }
				}

				delivery, err := dd.getDelivery(ctx, tgt)
				if err != nil {
					return wrapErr(err)
				}

				if err := delivery.AddRcpt(ctx, to); err != nil {
					return wrapErr(err)
				}
				delivery.recipients = append(delivery.recipients, originalTo)
			}
		}
	}

	return nil
}

func (dd *msgpipelineDelivery) Body(ctx context.Context, header textproto.Header, body buffer.Buffer) error {
	if err := dd.checkRunner.checkBody(ctx, dd.d.globalChecks, header, body); err != nil {
		return err
	}
	if err := dd.checkRunner.checkBody(ctx, dd.sourceBlock.checks, header, body); err != nil {
		return err
	}
	for blk := range dd.rcptModifiersState {
		if err := dd.checkRunner.checkBody(ctx, blk.checks, header, body); err != nil {
			return err
		}
	}

	if dd.d.FirstPipeline {
		// Add Received *after* checks to make sure they see the message literally
		// how we received it BUT place it below any other field that might be
		// added by applyResults (including Authentication-Results)
		// per recommendation in RFC 7001, Section 4 (see GH issue #135).
		received, err := target.GenerateReceived(ctx, dd.msgMeta, dd.d.Hostname, dd.msgMeta.OriginalFrom)
		if err != nil {
			return err
		}
		header.Add("Received", received)
	}

	if err := dd.checkRunner.applyResults(dd.d.Hostname, &header); err != nil {
		return err
	}

	// Run modifiers after Authentication-Results addition to make
	// sure signatures, etc will cover it.
	if err := dd.globalModifiersState.RewriteBody(ctx, &header, body); err != nil {
		return err
	}
	if err := dd.sourceModifiersState.RewriteBody(ctx, &header, body); err != nil {
		return err
	}
	for _, modifiers := range dd.rcptModifiersState {
		if err := modifiers.RewriteBody(ctx, &header, body); err != nil {
			return err
		}
	}

	for _, delivery := range dd.deliveries {
		if err := delivery.Body(ctx, header, body); err != nil {
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

func (dd *msgpipelineDelivery) BodyNonAtomic(ctx context.Context, c module.StatusCollector, header textproto.Header, body buffer.Buffer) {
	setStatusAll := func(err error) {
		for _, delivery := range dd.deliveries {
			for _, rcpt := range delivery.recipients {
				c.SetStatus(rcpt, err)
			}
		}
	}

	if err := dd.checkRunner.checkBody(ctx, dd.d.globalChecks, header, body); err != nil {
		setStatusAll(err)
		return
	}
	if err := dd.checkRunner.checkBody(ctx, dd.sourceBlock.checks, header, body); err != nil {
		setStatusAll(err)
		return
	}

	// Run modifiers after Authentication-Results addition to make
	// sure signatures, etc will cover it.
	if err := dd.globalModifiersState.RewriteBody(ctx, &header, body); err != nil {
		setStatusAll(err)
		return
	}
	if err := dd.sourceModifiersState.RewriteBody(ctx, &header, body); err != nil {
		setStatusAll(err)
		return
	}
	for _, modifiers := range dd.rcptModifiersState {
		if err := modifiers.RewriteBody(ctx, &header, body); err != nil {
			setStatusAll(err)
			return
		}
	}

	for _, delivery := range dd.deliveries {
		partDelivery, ok := delivery.Delivery.(module.PartialDelivery)
		if ok {
			partDelivery.BodyNonAtomic(ctx, statusCollector{
				originalRcpts: dd.msgMeta.OriginalRcpts,
				wrapped:       c,
			}, header, body)
			continue
		}

		if err := delivery.Body(ctx, header, body); err != nil {
			for _, rcpt := range delivery.recipients {
				c.SetStatus(rcpt, err)
			}
		}
	}
}

func (dd msgpipelineDelivery) Commit(ctx context.Context) error {
	dd.close()

	for _, delivery := range dd.deliveries {
		if err := delivery.Commit(ctx); err != nil {
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

func (dd msgpipelineDelivery) Abort(ctx context.Context) error {
	dd.close()

	var lastErr error
	for _, delivery := range dd.deliveries {
		if err := delivery.Abort(ctx); err != nil {
			dd.log.Debugf("delivery.Abort failure, Delivery object = %T: %v", delivery, err)
			lastErr = err
			// Continue anyway and try to Abort all remaining delivery objects.
		}
	}
	return lastErr
}

func (dd *msgpipelineDelivery) rcptBlockForAddr(ctx context.Context, rcptTo string) (*rcptBlock, error) {
	cleanRcpt, err := address.ForLookup(rcptTo)
	if err != nil {
		return nil, &exterrors.SMTPError{
			Code:         553,
			EnhancedCode: exterrors.EnhancedCode{5, 1, 2},
			Message:      "Unable to normalize the recipient address",
			Err:          err,
		}
	}

	for _, rcptIn := range dd.sourceBlock.rcptIn {
		_, ok, err := rcptIn.t.Lookup(ctx, cleanRcpt)
		if err != nil {
			dd.log.Error("destination_in lookup failed", err, "key", cleanRcpt)
			continue
		}
		if !ok {
			continue
		}
		return rcptIn.block, nil
	}

	// First try to match against complete address.
	rcptBlock, ok := dd.sourceBlock.perRcpt[cleanRcpt]
	if !ok {
		// Then try domain-only.
		_, domain, err := address.Split(cleanRcpt)
		if err != nil {
			return nil, &exterrors.SMTPError{
				Code:         501,
				EnhancedCode: exterrors.EnhancedCode{5, 1, 3},
				Message:      "Invalid recipient address",
				Err:          err,
				Reason:       "Can't extract local-part and host-part",
			}
		}

		// domain is already case-folded and normalized because it is a part of
		// cleanRcpt.
		rcptBlock, ok = dd.sourceBlock.perRcpt[domain]
		if !ok {
			// Fallback to the default source block.
			rcptBlock = dd.sourceBlock.defaultRcpt
			dd.log.Debugf("recipient %s matched by default rule (clean = %s)", rcptTo, cleanRcpt)
		} else {
			dd.log.Debugf("recipient %s matched by domain rule '%s'", rcptTo, domain)
		}
	} else {
		dd.log.Debugf("recipient %s matched by address rule '%s'", rcptTo, cleanRcpt)
	}
	return rcptBlock, nil
}

func (dd *msgpipelineDelivery) getRcptModifiers(ctx context.Context, rcptBlock *rcptBlock, rcptTo string) (module.ModifierState, error) {
	rcptModifiersState, ok := dd.rcptModifiersState[rcptBlock]
	if ok {
		return rcptModifiersState, nil
	}

	rcptModifiersState, err := rcptBlock.modifiers.ModStateForMsg(ctx, dd.msgMeta)
	if err != nil {
		return nil, err
	}

	newSender, err := rcptModifiersState.RewriteSender(ctx, dd.sourceAddr)
	if err == nil && newSender != dd.sourceAddr {
		dd.log.Msg("Per-recipient modifier changed sender address. This is not supported and will "+
			"be ignored.", "rcpt", rcptTo, "originalFrom", dd.sourceAddr, "modifiedFrom", newSender)
	}

	dd.rcptModifiersState[rcptBlock] = rcptModifiersState
	return rcptModifiersState, nil
}

func (dd *msgpipelineDelivery) getDelivery(ctx context.Context, tgt module.DeliveryTarget) (*delivery, error) {
	delivery_, ok := dd.deliveries[tgt]
	if ok {
		return delivery_, nil
	}

	deliveryObj, err := tgt.Start(ctx, dd.msgMeta, dd.sourceAddr)
	if err != nil {
		dd.log.Debugf("tgt.Start(%s) failure, target = %s: %v", dd.sourceAddr, objectName(tgt), err)
		return nil, err
	}
	delivery_ = &delivery{Delivery: deliveryObj}

	dd.log.Debugf("tgt.Start(%s) ok, target = %s", dd.sourceAddr, objectName(tgt))

	dd.deliveries[tgt] = delivery_
	return delivery_, nil
}

// Mock returns a MsgPipeline that merely delivers messages to a specified target
// and runs a set of checks.
//
// It is meant for use in tests for modules that embed a pipeline object.
func Mock(tgt module.DeliveryTarget, globalChecks []module.Check) *MsgPipeline {
	return &MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			globalChecks: globalChecks,
			perSource:    map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{tgt},
				},
			},
		},
	}
}
