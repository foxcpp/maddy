package maddy

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/maddy/buffer"
	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/log"
	"github.com/emersion/maddy/module"
)

// Dispatcher is a object that is responsible for selecting delivery targets
// for the message and running necessary checks and modificators.
//
// It implements module.DeliveryTarget.
//
// It is not a "module object" and is intended to be used as part of message
// source (Submission, SMTP, JMAP modules) implementation.
type Dispatcher struct {
	dispatcherCfg

	Log log.Logger
}

type sourceBlock struct {
	checks      CheckGroup
	rejectErr   error
	perRcpt     map[string]*rcptBlock
	defaultRcpt *rcptBlock
}

type rcptBlock struct {
	checks    CheckGroup
	rejectErr error
	targets   []module.DeliveryTarget
}

func NewDispatcher(globals map[string]interface{}, cfg []config.Node) (*Dispatcher, error) {
	parsedCfg, err := parseDispatcherRootCfg(globals, cfg)
	return &Dispatcher{
		dispatcherCfg: parsedCfg,
	}, err
}

func splitAddress(addr string) (mailbox, domain string, err error) {
	parts := strings.Split(addr, "@")
	switch len(parts) {
	case 1:
		if strings.EqualFold(parts[0], "postmaster") {
			return parts[0], "", nil
		}
		return "", "", fmt.Errorf("malformed address")
	case 2:
		if len(parts[0]) == 0 || len(parts[1]) == 0 {
			return "", "", fmt.Errorf("malformed address")
		}
		return parts[0], parts[1], nil
	default:
		return "", "", fmt.Errorf("malformed address")
	}
}

func (d *Dispatcher) Start(msgMeta *module.MsgMetadata, mailFrom string) (module.Delivery, error) {
	dd := dispatcherDelivery{
		rcptChecksState: make(map[*rcptBlock]module.CheckState),
		deliveries:      make(map[module.DeliveryTarget]module.Delivery),
		msgMeta:         msgMeta,
		cancelCtx:       context.Background(),
		log:             &d.Log,
	}

	globalChecksState, err := d.globalChecks.NewMessage(msgMeta)
	if err != nil {
		return nil, err
	}

	if err := globalChecksState.CheckConnection(dd.cancelCtx); err != nil {
		return nil, err
	}
	if err := globalChecksState.CheckSender(dd.cancelCtx, mailFrom); err != nil {
		return nil, err
	}
	dd.globalChecksState = globalChecksState

	// TODO: Init global modificators, run RewriteSender.

	// First try to match against complete address.
	sourceBlock, ok := d.perSource[strings.ToLower(mailFrom)]
	if !ok {
		// Then try domain-only.
		_, domain, err := splitAddress(mailFrom)
		if err != nil {
			return nil, err
		}

		sourceBlock, ok = d.perSource[strings.ToLower(domain)]
		if !ok {
			// Fallback to the default source block.
			sourceBlock = d.defaultSource
			dd.log.Debugf("sender %s matched by default rule (msg ID = %s)", mailFrom, msgMeta.ID)
		} else {
			dd.log.Debugf("sender %s matched by domain rule '%s' (msg ID = %s)", mailFrom, strings.ToLower(domain), msgMeta.ID)
		}
	} else {
		dd.log.Debugf("sender %s matched by address rule '%s' (msg ID = %s)", mailFrom, strings.ToLower(mailFrom), msgMeta.ID)
	}

	if sourceBlock.rejectErr != nil {
		dd.log.Debugf("sender %s rejected with error: %v (msg ID = %s)", mailFrom, sourceBlock.rejectErr, msgMeta.ID)
		return nil, sourceBlock.rejectErr
	}
	dd.sourceBlock = sourceBlock

	sourceChecksState, err := sourceBlock.checks.NewMessage(msgMeta)
	if err != nil {
		return nil, err
	}
	if err := sourceChecksState.CheckConnection(dd.cancelCtx); err != nil {
		return nil, err
	}
	if err := sourceChecksState.CheckSender(dd.cancelCtx, mailFrom); err != nil {
		return nil, err
	}
	dd.sourceChecksState = sourceChecksState

	// TODO: Init per-sender modificators, run RewriteSender.

	dd.sourceAddr = mailFrom

	return dd, nil
}

type dispatcherDelivery struct {
	globalChecksState module.CheckState
	sourceChecksState module.CheckState
	rcptChecksState   map[*rcptBlock]module.CheckState

	log *log.Logger

	sourceAddr  string
	sourceBlock sourceBlock

	deliveries map[module.DeliveryTarget]module.Delivery
	msgMeta    *module.MsgMetadata
	cancelCtx  context.Context
}

func (dd dispatcherDelivery) AddRcpt(to string) error {
	// First try to match against complete address.
	rcptBlock, ok := dd.sourceBlock.perRcpt[strings.ToLower(to)]
	if !ok {
		// Then try domain-only.
		_, domain, err := splitAddress(to)
		if err != nil {
			return err
		}

		rcptBlock, ok = dd.sourceBlock.perRcpt[strings.ToLower(domain)]
		if !ok {
			// Fallback to the default source block.
			rcptBlock = dd.sourceBlock.defaultRcpt
			dd.log.Debugf("recipient %s matched by default rule (msg ID = %s)", to, dd.msgMeta.ID)
		} else {
			dd.log.Debugf("recipient %s matched by domain rule '%s' (msg ID = %s)", to, strings.ToLower(domain), dd.msgMeta.ID)
		}
	} else {
		dd.log.Debugf("recipient %s matched by address rule '%s' (msg ID = %s)", to, strings.ToLower(to), dd.msgMeta.ID)
	}

	if rcptBlock.rejectErr != nil {
		dd.log.Debugf("recipient %s rejected: %v (msg ID = %s)", to, rcptBlock.rejectErr, dd.msgMeta.ID)
		return rcptBlock.rejectErr
	}

	var rcptChecksState module.CheckState
	if rcptChecksState, ok = dd.rcptChecksState[rcptBlock]; !ok {
		var err error
		rcptChecksState, err = rcptBlock.checks.NewMessage(dd.msgMeta)
		if err != nil {
			return err
		}
		if err := rcptChecksState.CheckConnection(dd.cancelCtx); err != nil {
			return err
		}
		if err := rcptChecksState.CheckSender(dd.cancelCtx, dd.sourceAddr); err != nil {
			return err
		}
		dd.rcptChecksState[rcptBlock] = rcptChecksState
	}

	if err := rcptChecksState.CheckRcpt(dd.cancelCtx, to); err != nil {
		return err
	}

	// TODO: Init per-rcpt modificators, run RewriteRcpt.

	for _, target := range rcptBlock.targets {
		var delivery module.Delivery
		var err error
		if delivery, ok = dd.deliveries[target]; !ok {
			delivery, err = target.Start(dd.msgMeta, dd.sourceAddr)
			if err != nil {
				dd.log.Debugf("target.Start(%s) failure, target = %s (%s): %v (msg ID = %s)",
					dd.sourceAddr, target.(module.Module).InstanceName(), target.(module.Module).Name(), err, dd.msgMeta.ID)
				return err
			}

			dd.log.Debugf("target.Start(%s) ok, target = %s (%s) (msg ID = %s)",
				dd.sourceAddr, target.(module.Module).InstanceName(), target.(module.Module).Name(), dd.msgMeta.ID)

			dd.deliveries[target] = delivery
		}

		if err := delivery.AddRcpt(to); err != nil {
			dd.log.Debugf("delivery.AddRcpt(%s) failure, Delivery object = %T: %v (msg ID = %s)",
				to, delivery, err, dd.msgMeta.ID)
			return err
		}
		dd.log.Debugf("delivery.AddRcpt(%s) ok, Delivery object = %T (msg ID = %v)",
			to, delivery, dd.msgMeta.ID)
	}

	return nil
}

func (dd dispatcherDelivery) Body(header textproto.Header, body buffer.Buffer) error {
	var headerLock sync.RWMutex
	if err := dd.globalChecksState.CheckBody(dd.cancelCtx, &headerLock, header, body); err != nil {
		return err
	}

	for _, delivery := range dd.deliveries {
		if err := delivery.Body(header, body); err != nil {
			dd.log.Debugf("delivery.Body failure, Delivery object = %T: %v (msg ID = %s)",
				delivery, err, dd.msgMeta.ID)
			return err
		}
		dd.log.Debugf("delivery.Body ok, Delivery object = %T (msg ID = %v)",
			delivery, dd.msgMeta.ID)
	}
	return nil
}

func (dd dispatcherDelivery) Commit() error {
	for _, delivery := range dd.deliveries {
		if err := delivery.Commit(); err != nil {
			dd.log.Debugf("delivery.Commit failure, Delivery object = %T: %v (msg ID = %s)",
				delivery, err, dd.msgMeta.ID)
			// No point in Committing remaining deliveries, everything is broken already.
			return err
		}
		dd.log.Debugf("delivery.Commit ok, Delivery object = %T (msg ID = %s)",
			delivery, dd.msgMeta.ID)
	}
	return nil
}

func (dd dispatcherDelivery) Abort() error {
	var lastErr error
	for _, delivery := range dd.deliveries {
		if err := delivery.Abort(); err != nil {
			dd.log.Debugf("delivery.Abort failure, Delivery object = %T: %v (msg ID = %s)",
				delivery, err, dd.msgMeta.ID)
			lastErr = err
			// Continue anyway and try to Abort all remaining delivery objects.
		}
		dd.log.Debugf("delivery.Abort ok, Delivery object = %T (msg ID = %s)",
			delivery, dd.msgMeta.ID)
	}
	dd.log.Debugf("delivery aborted (msg ID = %s)", dd.msgMeta.ID)
	return lastErr
}
