package maddy

import (
	"fmt"
	"strings"

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
	// Configuration.
	// TODO: globalChecks
	perSource     map[string]sourceBlock
	defaultSource sourceBlock

	Log log.Logger
}

type sourceBlock struct {
	rejectErr   error
	perRcpt     map[string]*rcptBlock
	defaultRcpt *rcptBlock
}

type rcptBlock struct {
	rejectErr error
	targets   []module.DeliveryTarget
}

func NewDispatcher(cfg []config.Node) (Dispatcher, error) {
	return Dispatcher{}, nil
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

func (d *Dispatcher) Start(ctx *module.DeliveryContext, mailFrom string) (module.Delivery, error) {
	dd := dispatcherDelivery{
		deliveries: make(map[module.DeliveryTarget]module.Delivery),
		ctx:        ctx,
		log:        &d.Log,
	}

	// TODO: Init global checks, run CheckConnection and CheckSender.
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
			dd.log.Debugf("sender %s matched by default rule (delivery ID = %s)", mailFrom, ctx.DeliveryID)
		} else {
			dd.log.Debugf("sender %s matched by domain rule '%s' (delivery ID = %s)", mailFrom, strings.ToLower(domain), ctx.DeliveryID)
		}
	} else {
		dd.log.Debugf("sender %s matched by address rule '%s' (delivery ID = %s)", mailFrom, strings.ToLower(mailFrom), ctx.DeliveryID)
	}

	if sourceBlock.rejectErr != nil {
		dd.log.Printf("sender %s rejected with error: %v (delivery ID = %s)", mailFrom, sourceBlock.rejectErr, ctx.DeliveryID)
		return nil, sourceBlock.rejectErr
	}
	dd.sourceBlock = sourceBlock

	// TODO: Init per-sender checks, run CheckConnection, CheckSender.
	// TODO: Init per-sender modificators, run RewriteSender.

	dd.sourceAddr = mailFrom

	return dd, nil
}

type dispatcherDelivery struct {
	// TODO: globalChecksState
	// TODO: sourceChecksState
	// TODO: rcptChecksState

	log *log.Logger

	sourceAddr  string
	sourceBlock sourceBlock

	deliveries map[module.DeliveryTarget]module.Delivery
	ctx        *module.DeliveryContext
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
			dd.log.Debugf("recipient %s matched by default rule (delivery ID = %s)", to, dd.ctx.DeliveryID)
		} else {
			dd.log.Debugf("recipient %s matched by domain rule '%s' (delivery ID = %s)", to, strings.ToLower(domain), dd.ctx.DeliveryID)
		}
	} else {
		dd.log.Debugf("recipient %s matched by address rule '%s' (delivery ID = %s)", to, strings.ToLower(to), dd.ctx.DeliveryID)
	}

	if rcptBlock.rejectErr != nil {
		dd.log.Printf("recipient %s rejected: %v (delivery ID = %s)", to, rcptBlock.rejectErr, dd.ctx.DeliveryID)
		return rcptBlock.rejectErr
	}

	// TODO: Init per-rcpt checks, run CheckConnection, CheckSender and CheckRcpt.
	// TODO: Init per-rcpt modificators, run RewriteRcpt.

	for _, target := range rcptBlock.targets {
		var delivery module.Delivery
		var err error
		if delivery, ok = dd.deliveries[target]; !ok {
			delivery, err = target.Start(dd.ctx, dd.sourceAddr)
			if err != nil {
				dd.log.Printf("target.Start(%s) failure, target = %s (%s): %v (delivery ID = %s)",
					dd.sourceAddr, target.(module.Module).InstanceName(), target.(module.Module).Name(), err, dd.ctx.DeliveryID)
				return err
			}

			dd.log.Debugf("target.Start(%s) ok, target = %s (%s) (delivery ID = %s)",
				dd.sourceAddr, target.(module.Module).InstanceName(), target.(module.Module).Name(), dd.ctx.DeliveryID)

			dd.deliveries[target] = delivery
		}

		if err := delivery.AddRcpt(to); err != nil {
			dd.log.Printf("delivery.AddRcpt(%s) failure, Delivery object = %T: %v (delivery ID = %s)",
				to, delivery, err, dd.ctx.DeliveryID)
			return err
		}
		dd.log.Debugf("delivery.AddRcpt(%s) ok, Delivery object = %T (delivery ID = %v)",
			to, delivery, dd.ctx.DeliveryID)
	}

	return nil
}

func (dd dispatcherDelivery) Body(body module.BodyBuffer) error {
	for _, delivery := range dd.deliveries {
		if err := delivery.Body(body); err != nil {
			dd.log.Printf("delivery.Body failure, Delivery object = %T: %v (delivery ID = %s)",
				delivery, err, dd.ctx.DeliveryID)
			return err
		}
		dd.log.Debugf("delivery.Body ok, Delivery object = %T (delivery ID = %v)",
			delivery, dd.ctx.DeliveryID)
	}
	return nil
}

func (dd dispatcherDelivery) Commit() error {
	for _, delivery := range dd.deliveries {
		if err := delivery.Commit(); err != nil {
			dd.log.Printf("delivery.Commit failure, Delivery object = %T: %v (delivery ID = %s)",
				delivery, err, dd.ctx.DeliveryID)
			// No point in Committing remaining deliveries, everything is broken already.
			return err
		}
		dd.log.Debugf("delivery.Commit ok, Delivery object = %T (delivery ID = %s)",
			delivery, dd.ctx.DeliveryID)
	}
	return nil
}

func (dd dispatcherDelivery) Abort() error {
	var lastErr error
	for _, delivery := range dd.deliveries {
		if err := delivery.Abort(); err != nil {
			dd.log.Printf("delivery.Abort failure, Delivery object = %T: %v (delivery ID = %s)",
				delivery, err, dd.ctx.DeliveryID)
			lastErr = err
			// Continue anyway and try to Abort all remaining delivery objects.
		}
		dd.log.Debugf("delivery.Abort ok, Delivery object = %T (delivery ID = %s)",
			delivery, dd.ctx.DeliveryID)
	}
	dd.log.Printf("delivery aborted (delivery ID = %s)", dd.ctx.DeliveryID)
	return lastErr
}
