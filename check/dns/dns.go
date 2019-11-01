package dns

import (
	"context"
	"net"
	"strings"

	"github.com/foxcpp/maddy/address"
	"github.com/foxcpp/maddy/check"
	"github.com/foxcpp/maddy/exterrors"
	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/module"
)

func requireMatchingRDNS(ctx check.StatelessCheckContext) module.CheckResult {
	tcpAddr, ok := ctx.MsgMeta.SrcAddr.(*net.TCPAddr)
	if !ok {
		log.Debugf("non TCP/IP source, skipped")
		return module.CheckResult{}
	}

	names, err := ctx.Resolver.LookupAddr(context.Background(), tcpAddr.IP.String())
	if err != nil || len(names) == 0 {
		return module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:         550,
				EnhancedCode: exterrors.EnhancedCode{5, 7, 25},
				Message:      "rDNS lookup failure during policy check",
				CheckName:    "require_matching_rdns",
				Err:          err,
			},
		}
	}

	srcDomain := strings.TrimRight(ctx.MsgMeta.SrcHostname, ".")

	for _, name := range names {
		if strings.TrimRight(name, ".") == srcDomain {
			ctx.Logger.Debugf("PTR record %s matches source domain, OK", name)
			return module.CheckResult{}
		}
	}
	return module.CheckResult{
		Reason: &exterrors.SMTPError{
			Code:         550,
			EnhancedCode: exterrors.EnhancedCode{5, 7, 25},
			Message:      "rDNS name does not match source hostname",
			CheckName:    "require_matching_rdns",
		},
	}
}

func requireMXRecord(ctx check.StatelessCheckContext, mailFrom string) module.CheckResult {
	_, domain, err := address.Split(mailFrom)
	if err != nil {
		return module.CheckResult{
			Reason: exterrors.WithFields(err, map[string]interface{}{
				"check": "require_matching_rdns",
			}),
		}
	}
	if domain == "" {
		return module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:         501,
				EnhancedCode: exterrors.EnhancedCode{5, 1, 8},
				Message:      "No domain part",
				CheckName:    "require_matching_rdns",
			},
		}
	}

	_, ok := ctx.MsgMeta.SrcAddr.(*net.TCPAddr)
	if !ok {
		ctx.Logger.Println("non-TCP/IP source")
		return module.CheckResult{}
	}

	srcMx, err := ctx.Resolver.LookupMX(context.Background(), domain)
	if err != nil {
		return module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:         501,
				EnhancedCode: exterrors.EnhancedCode{5, 7, 27},
				Message:      "Could not find MX records for MAIL FROM domain",
				CheckName:    "require_matching_rdns",
				Err:          err,
			},
		}
	}

	if len(srcMx) == 0 {
		return module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:         501,
				EnhancedCode: exterrors.EnhancedCode{5, 7, 27},
				Message:      "Domain in MAIL FROM has no MX records",
				CheckName:    "require_matching_rdns",
			},
		}
	}

	return module.CheckResult{}
}

func requireMatchingEHLO(ctx check.StatelessCheckContext) module.CheckResult {
	tcpAddr, ok := ctx.MsgMeta.SrcAddr.(*net.TCPAddr)
	if !ok {
		ctx.Logger.Printf("non-TCP/IP source, skipped")
		return module.CheckResult{}
	}

	srcIPs, err := ctx.Resolver.LookupIPAddr(context.Background(), ctx.MsgMeta.SrcHostname)
	if err != nil {
		return module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:         550,
				EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
				Message:      "DNS lookup failure during policy check",
				CheckName:    "require_matching_ehlo",
				Err:          err,
			},
		}
	}

	for _, ip := range srcIPs {
		if tcpAddr.IP.Equal(ip.IP) {
			ctx.Logger.Debugf("A/AAA record found for %s for %s domain", tcpAddr.IP, ctx.MsgMeta.SrcHostname)
			return module.CheckResult{}
		}
	}
	return module.CheckResult{
		Reason: &exterrors.SMTPError{
			Code:         550,
			EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
			Message:      "No matching A/AAA records found for EHLO hostname",
			CheckName:    "require_matching_ehlo",
		},
	}
}

func init() {
	check.RegisterStatelessCheck("require_matching_rdns", check.FailAction{Quarantine: true},
		requireMatchingRDNS, nil, nil, nil)
	check.RegisterStatelessCheck("require_mx_record", check.FailAction{Quarantine: true},
		nil, requireMXRecord, nil, nil)
	check.RegisterStatelessCheck("require_matching_ehlo", check.FailAction{Quarantine: true},
		requireMatchingEHLO, nil, nil, nil)
}
