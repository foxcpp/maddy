package dns

import (
	"context"
	"net"
	"strings"

	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/address"
	"github.com/foxcpp/maddy/check"
	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/module"
)

func requireMatchingRDNS(ctx check.StatelessCheckContext) module.CheckResult {
	tcpAddr, ok := ctx.MsgMeta.SrcAddr.(*net.TCPAddr)
	if !ok {
		log.Debugf("non TCP/IP source (%v), skipped", ctx.MsgMeta.SrcAddr)
		return module.CheckResult{}
	}

	names, err := ctx.Resolver.LookupAddr(context.Background(), tcpAddr.IP.String())
	if err != nil || len(names) == 0 {
		ctx.Logger.Println(err)
		return module.CheckResult{
			RejectErr: &smtp.SMTPError{
				Code:         550,
				EnhancedCode: smtp.EnhancedCode{5, 7, 25},
				Message:      "could look up rDNS address for source",
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
	ctx.Logger.Printf("no PTR records for %v IP pointing to %s", tcpAddr.IP, srcDomain)
	return module.CheckResult{
		RejectErr: &smtp.SMTPError{
			Code:         550,
			EnhancedCode: smtp.EnhancedCode{5, 7, 25},
			Message:      "rDNS name does not match source hostname",
		},
	}
}

func requireMXRecord(ctx check.StatelessCheckContext, mailFrom string) module.CheckResult {
	_, domain, err := address.Split(mailFrom)
	if err != nil {
		return module.CheckResult{RejectErr: err}
	}
	if domain == "" {
		// TODO: Make it configurable whether <postmaster> is allowed.
		return module.CheckResult{
			RejectErr: &smtp.SMTPError{
				Code:         501,
				EnhancedCode: smtp.EnhancedCode{5, 1, 8},
				Message:      "<postmaster> is not allowed",
			},
		}
	}

	_, ok := ctx.MsgMeta.SrcAddr.(*net.TCPAddr)
	if !ok {
		ctx.Logger.Debugf("not TCP/IP source (%v), skipped", ctx.MsgMeta.SrcAddr)
		return module.CheckResult{}
	}

	srcMx, err := ctx.Resolver.LookupMX(context.Background(), domain)
	if err != nil {
		ctx.Logger.Println(err)
		return module.CheckResult{
			RejectErr: &smtp.SMTPError{
				Code:         501,
				EnhancedCode: smtp.EnhancedCode{5, 7, 27},
				Message:      "could not find MX records for MAIL FROM domain",
			},
		}
	}

	if len(srcMx) == 0 {
		ctx.Logger.Printf("%s got no MX records", domain)
		return module.CheckResult{
			RejectErr: &smtp.SMTPError{
				Code:         501,
				EnhancedCode: smtp.EnhancedCode{5, 7, 27},
				Message:      "domain in MAIL FROM has no MX records",
			},
		}
	}

	return module.CheckResult{}
}

func requireMatchingEHLO(ctx check.StatelessCheckContext) module.CheckResult {
	tcpAddr, ok := ctx.MsgMeta.SrcAddr.(*net.TCPAddr)
	if !ok {
		ctx.Logger.Debugf("not TCP/IP source (%v), skipped", ctx.MsgMeta.SrcAddr)
		return module.CheckResult{}
	}

	srcIPs, err := ctx.Resolver.LookupIPAddr(context.Background(), ctx.MsgMeta.SrcHostname)
	if err != nil {
		ctx.Logger.Println(err)
		// TODO: Check whether lookup is failed due to temporary error and reject with 4xx code.
		return module.CheckResult{
			RejectErr: &smtp.SMTPError{
				Code:         550,
				EnhancedCode: smtp.EnhancedCode{5, 7, 0},
				Message:      "DNS lookup failure during policy check",
			},
		}
	}

	for _, ip := range srcIPs {
		if tcpAddr.IP.Equal(ip.IP) {
			ctx.Logger.Debugf("A/AAA record found for %s for %s domain", tcpAddr.IP, ctx.MsgMeta.SrcHostname)
			return module.CheckResult{}
		}
	}
	ctx.Logger.Printf("no A/AAA records found for %s for %s domain", tcpAddr.IP, ctx.MsgMeta.SrcHostname)
	return module.CheckResult{
		RejectErr: &smtp.SMTPError{
			Code:         550,
			EnhancedCode: smtp.EnhancedCode{5, 7, 0},
			Message:      "no matching A/AAA records found for EHLO hostname",
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
