package maddy

import (
	"context"
	"net"
	"strings"

	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/log"
)

func requireMatchingRDNS(ctx StatelessCheckContext) error {
	tcpAddr, ok := ctx.MsgMeta.SrcAddr.(*net.TCPAddr)
	if !ok {
		log.Debugf("non TCP/IP source (%v), skipped", ctx.MsgMeta.SrcAddr)
		return nil
	}

	names, err := ctx.Resolver.LookupAddr(context.Background(), tcpAddr.IP.String())
	if err != nil || len(names) == 0 {
		ctx.Logger.Printf("rDNS query for %v failed: %v", tcpAddr.IP, err)
		return &smtp.SMTPError{
			Code:         550,
			EnhancedCode: smtp.EnhancedCode{5, 7, 25},
			Message:      "could look up rDNS address for source",
		}
	}

	srcDomain := strings.TrimRight(ctx.MsgMeta.SrcHostname, ".")

	for _, name := range names {
		if strings.TrimRight(name, ".") == srcDomain {
			ctx.Logger.Debugf("PTR record %s matches source domain, OK", name)
			return nil
		}
	}
	ctx.Logger.Printf("no PTR records for %v IP pointing to %s", tcpAddr.IP, srcDomain)
	return &smtp.SMTPError{
		Code:         550,
		EnhancedCode: smtp.EnhancedCode{5, 7, 25},
		Message:      "rDNS name does not match source hostname",
	}
}

func requireMXRecord(ctx StatelessCheckContext, mailFrom string) error {
	_, domain, err := splitAddress(mailFrom)
	if err != nil {
		return err
	}
	if domain == "" {
		// TODO: Make it configurable whether <postmaster> is allowed.
		return &smtp.SMTPError{
			Code:         501,
			EnhancedCode: smtp.EnhancedCode{5, 1, 8},
			Message:      "<postmaster> is not allowed",
		}
	}

	_, ok := ctx.MsgMeta.SrcAddr.(*net.TCPAddr)
	if !ok {
		ctx.Logger.Debugf("not TCP/IP source (%v), skipped", ctx.MsgMeta.SrcAddr)
		return nil
	}

	srcMx, err := ctx.Resolver.LookupMX(context.Background(), domain)
	if err != nil {
		ctx.Logger.Debugf("%s does not resolve", domain)
		return &smtp.SMTPError{
			Code:         501,
			EnhancedCode: smtp.EnhancedCode{5, 7, 27},
			Message:      "could not find MX records for MAIL FROM domain",
		}
	}

	if len(srcMx) == 0 {
		return &smtp.SMTPError{
			Code:         501,
			EnhancedCode: smtp.EnhancedCode{5, 7, 27},
			Message:      "domain in MAIL FROM has no MX records",
		}
	}

	return nil
}

func requireMatchingEHLO(ctx StatelessCheckContext) error {
	tcpAddr, ok := ctx.MsgMeta.SrcAddr.(*net.TCPAddr)
	if !ok {
		ctx.Logger.Debugf("not TCP/IP source (%v), skipped", ctx.MsgMeta.SrcAddr)
		return nil
	}

	srcIPs, err := ctx.Resolver.LookupIPAddr(context.Background(), ctx.MsgMeta.SrcHostname)
	if err != nil {
		ctx.Logger.Printf("%s does not resolve: %v", ctx.MsgMeta.SrcHostname, err)
		// TODO: Check whether lookup is failed due to temporary error and reject with 4xx code.
		return &smtp.SMTPError{
			Code:         550,
			EnhancedCode: smtp.EnhancedCode{5, 7, 0},
			Message:      "DNS lookup failure during policy check",
		}
	}

	for _, ip := range srcIPs {
		if tcpAddr.IP.Equal(ip.IP) {
			ctx.Logger.Debugf("A/AAA record found for %s for %s domain", tcpAddr.IP, ctx.MsgMeta.SrcHostname)
			return nil
		}
	}
	ctx.Logger.Printf("no A/AAA records found for %s for %s domain", tcpAddr.IP, ctx.MsgMeta.SrcHostname)
	return &smtp.SMTPError{
		Code:         550,
		EnhancedCode: smtp.EnhancedCode{5, 7, 0},
		Message:      "no matching A/AAA records found for EHLO hostname",
	}
}

func init() {
	RegisterStatelessCheck("require_matching_rdns", checkFailAction{quarantine: true},
		requireMatchingRDNS, nil, nil, nil)
	RegisterStatelessCheck("require_mx_record", checkFailAction{quarantine: true},
		nil, requireMXRecord, nil, nil)
	RegisterStatelessCheck("require_matching_ehlo", checkFailAction{quarantine: true},
		requireMatchingEHLO, nil, nil, nil)
}
