package maddy

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"

	"github.com/emersion/maddy/log"
	"github.com/emersion/maddy/module"
)

func checkSrcRDNS(r Resolver, ctx *module.DeliveryContext, _ io.Reader) (io.Reader, error) {
	tcpAddr, ok := ctx.SrcAddr.(*net.TCPAddr)
	if !ok {
		log.Debugf("check_source_rdns: non TCP/IP source (%v), skipped", ctx.SrcAddr)
		return nil, nil
	}

	names, err := r.LookupAddr(context.Background(), tcpAddr.IP.String())
	if err != nil || len(names) == 0 {
		log.Printf("check_source_rdns: rDNS query for %v failed (%v), FAIL, delivery ID = %s", tcpAddr.IP, err, ctx.DeliveryID)
		return nil, errors.New("could look up rDNS address for source")
	}

	srcDomain := strings.TrimRight(ctx.SrcHostname, ".")

	for _, name := range names {
		if strings.TrimRight(name, ".") == srcDomain {
			log.Debugf("check_source_rdns: PTR record %s matches source domain, OK, delivery ID = %s", name, ctx.DeliveryID)
			return nil, nil
		}
	}
	log.Printf("check_source_rdns: no PTR records for %v IP pointing to %s, FAIL, delivery ID = %s", tcpAddr.IP, srcDomain, ctx.DeliveryID)
	return nil, errors.New("rDNS name does not match source hostname")
}

func checkSrcMX(r Resolver, ctx *module.DeliveryContext, _ io.Reader) (io.Reader, error) {
	parts := strings.Split(ctx.From, "@")
	if len(parts) != 2 {
		return nil, errors.New("malformed address")
	}
	domain := parts[1]

	tcpAddr, ok := ctx.SrcAddr.(*net.TCPAddr)
	if !ok {
		log.Debugf("check_source_mx: not TCP/IP source (%v), skipped", ctx.SrcAddr)
		return nil, nil
	}

	srcMx, err := r.LookupMX(context.Background(), domain)
	if err != nil {
		log.Debugf("check_source_mx: %s does not resolve, FAIL, delivery ID = %s", domain, ctx.DeliveryID)
		return nil, errors.New("could not find MX records for from domain")
	}

	for _, mx := range srcMx {
		if mx.Host == ctx.SrcHostname || mx.Host == tcpAddr.IP.String() {
			log.Debugf("check_source_mx: MX record %s matches %v (%v), OK, delivery ID = %s", mx.Host, ctx.SrcHostname, tcpAddr.IP, ctx.DeliveryID)
			return nil, nil
		}
	}
	log.Printf("check_source_mx: no matching MX records for %s (%s), FAIL, delivery ID = %s", ctx.SrcHostname, tcpAddr.IP, ctx.DeliveryID)
	return nil, errors.New("From domain has no MX record for itself")
}

func checkSrcHostname(r Resolver, ctx *module.DeliveryContext, _ io.Reader) (io.Reader, error) {
	tcpAddr, ok := ctx.SrcAddr.(*net.TCPAddr)
	if !ok {
		log.Debugf("check_source_hostname: not TCP/IP source (%v), skipped", ctx.SrcAddr)
		return nil, nil
	}

	srcIPs, err := r.LookupIPAddr(context.Background(), ctx.SrcHostname)
	if err != nil {
		log.Printf("check_source_hostname: %s does not resolve, FAIL, delivery ID = %s", ctx.SrcHostname, ctx.DeliveryID)
		return nil, errors.New("source hostname does not resolve")
	}

	for _, ip := range srcIPs {
		if tcpAddr.IP.Equal(ip.IP) {
			log.Debugf("check_source_hostname: A/AAA record found for %s for %s domain, OK, delivery ID = %s", tcpAddr.IP, ctx.SrcHostname, ctx.DeliveryID)
			return nil, nil
		}
	}
	log.Printf("check_source_hostname: no A/AAA records found for %s for %s domain, FAIL, delivery ID = %s", tcpAddr.IP, ctx.SrcHostname, ctx.DeliveryID)
	return nil, errors.New("source hostname does not resolve to source address")
}

func init() {
	RegisterFuncFilter("check_source_rdns", "source_rdns", checkSrcRDNS)
	RegisterFuncFilter("check_source_mx", "source_mx", checkSrcMX)
	RegisterFuncFilter("check_source_hostname", "source_hostname", checkSrcHostname)
}
