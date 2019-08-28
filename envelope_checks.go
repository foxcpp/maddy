package maddy

import (
	"context"
	"errors"
	"net"
	"strings"

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
		return errors.New("could look up rDNS address for source")
	}

	srcDomain := strings.TrimRight(ctx.MsgMeta.SrcHostname, ".")

	for _, name := range names {
		if strings.TrimRight(name, ".") == srcDomain {
			ctx.Logger.Debugf("PTR record %s matches source domain, OK", name)
			return nil
		}
	}
	ctx.Logger.Printf("no PTR records for %v IP pointing to %s", tcpAddr.IP, srcDomain)
	// TODO: Use SMTPError with enhanced codes. Also for remaining checks.
	return errors.New("rDNS name does not match source hostname")
}

func requireMXRecord(ctx StatelessCheckContext, mailFrom string) error {
	_, domain, err := splitAddress(mailFrom)
	if err != nil {
		return err
	}
	if domain == "" {
		// TODO: Make it configurable whether <postmaster> is allowed.
		return errors.New("<postmaster> is not allowed")
	}

	_, ok := ctx.MsgMeta.SrcAddr.(*net.TCPAddr)
	if !ok {
		ctx.Logger.Debugf("not TCP/IP source (%v), skipped", ctx.MsgMeta.SrcAddr)
		return nil
	}

	srcMx, err := ctx.Resolver.LookupMX(context.Background(), domain)
	if err != nil {
		ctx.Logger.Debugf("%s does not resolve", domain)
		return errors.New("could not find MX records for MAIL FROM domain")
	}

	if len(srcMx) == 0 {
		return errors.New("domain in MAIL FROM has no MX records")
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
		ctx.Logger.Printf("%s does not resolve", ctx.MsgMeta.SrcHostname)
		return errors.New("source hostname does not resolve")
	}

	for _, ip := range srcIPs {
		if tcpAddr.IP.Equal(ip.IP) {
			ctx.Logger.Debugf("A/AAA record found for %s for %s domain", tcpAddr.IP, ctx.MsgMeta.SrcHostname)
			return nil
		}
	}
	ctx.Logger.Printf("no A/AAA records found for %s for %s domain", tcpAddr.IP, ctx.MsgMeta.SrcHostname)
	return errors.New("source hostname does not resolve to source address")
}

func init() {
	RegisterStatelessCheck("require_matching_rdns", checkFailAction{quarantine: true},
		requireMatchingRDNS, nil, nil, nil)
	RegisterStatelessCheck("require_mx_record", checkFailAction{quarantine: true},
		nil, requireMXRecord, nil, nil)
	RegisterStatelessCheck("require_matching_ehlo", checkFailAction{quarantine: true},
		requireMatchingEHLO, nil, nil, nil)
}
