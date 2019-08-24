package maddy

import (
	"context"
	"errors"
	"net"
	"strings"

	"github.com/emersion/maddy/log"
)

func checkSrcRDNS(ctx StatelessCheckContext) error {
	tcpAddr, ok := ctx.MsgMeta.SrcAddr.(*net.TCPAddr)
	if !ok {
		log.Debugf("non TCP/IP source (%v), skipped", ctx.MsgMeta.SrcAddr)
		return nil
	}

	names, err := ctx.Resolver.LookupAddr(context.Background(), tcpAddr.IP.String())
	if err != nil || len(names) == 0 {
		log.Printf("rDNS query for %v failed: %v", tcpAddr.IP, err)
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

func checkSrcMX(ctx StatelessCheckContext, mailFrom string) error {
	_, domain, err := splitAddress(mailFrom)
	if err != nil {
		return err
	}
	if domain == "" {
		// TODO: Make it configurable whether <postmaster> is allowed.
		return errors.New("check_source_mx: <postmaster> is not allowed")
	}

	tcpAddr, ok := ctx.MsgMeta.SrcAddr.(*net.TCPAddr)
	if !ok {
		ctx.Logger.Debugf("not TCP/IP source (%v), skipped", ctx.MsgMeta.SrcAddr)
		return nil
	}

	srcMx, err := ctx.Resolver.LookupMX(context.Background(), domain)
	if err != nil {
		ctx.Logger.Debugf("%s does not resolve", domain)
		return errors.New("could not find MX records for from domain")
	}

	for _, mx := range srcMx {
		if mx.Host == ctx.MsgMeta.SrcHostname || mx.Host == tcpAddr.IP.String() {
			ctx.Logger.Debugf("MX record %s matches %v (%v)", mx.Host, ctx.MsgMeta.SrcHostname, tcpAddr.IP)
			return nil
		}

		// MX record may contain hostname that will resolve to source hostname, check that too.
		addrs, err := ctx.Resolver.LookupIPAddr(context.Background(), mx.Host)
		if err != nil {
			// TODO: Actually check whether mx.Host is a domain.
			// Probably that was an IP.
			continue
		}

		for _, addr := range addrs {
			if addr.IP.Equal(tcpAddr.IP) {
				ctx.Logger.Debugf("MX record %s matches %v (%v), OK", mx.Host, ctx.MsgMeta.SrcHostname, tcpAddr.IP)
				return nil
			}
		}
	}
	ctx.Logger.Printf("no matching MX records for %s (%s)", ctx.MsgMeta.SrcHostname, tcpAddr.IP)
	return errors.New("domain in MAIL FROM has no MX record for itself")
}

func checkSrcHostname(ctx StatelessCheckContext) error {
	tcpAddr, ok := ctx.MsgMeta.SrcAddr.(*net.TCPAddr)
	if !ok {
		ctx.Logger.Debugf("not TCP/IP source (%v), skipped", ctx.MsgMeta.SrcAddr)
		return nil
	}

	srcIPs, err := ctx.Resolver.LookupIPAddr(context.Background(), ctx.MsgMeta.SrcHostname)
	if err != nil {
		log.Printf("%s does not resolve", ctx.MsgMeta.SrcHostname)
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
	RegisterStatelessCheck("check_source_rdns", checkSrcRDNS, nil, nil, nil)
	RegisterStatelessCheck("check_source_mx", nil, checkSrcMX, nil, nil)
	RegisterStatelessCheck("check_source_hostname", checkSrcHostname, nil, nil, nil)
}
