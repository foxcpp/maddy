package dnsbl

import (
	"context"
	"net"
	"strconv"
	"strings"

	"github.com/foxcpp/maddy/dns"
	"github.com/foxcpp/maddy/exterrors"
)

type ListedErr struct {
	Identity string
	List     string
	Reason   string
}

func (le ListedErr) Fields() map[string]interface{} {
	return map[string]interface{}{
		"check":           "dnsbl",
		"list":            le.List,
		"listed_identity": le.Identity,
		"reason":          le.Reason,
		"smtp_code":       554,
		"smtp_enchcode":   exterrors.EnhancedCode{5, 7, 0},
		"smtp_msg":        "Client identity listed in the used DNSBL",
	}
}

func (le ListedErr) Error() string {
	return le.Identity + " is listed in the used DNSBL"
}

func checkDomain(resolver dns.Resolver, cfg List, domain string) error {
	query := domain + "." + cfg.Zone

	addrs, err := resolver.LookupHost(context.Background(), query)
	if err != nil {
		if dnsErr, ok := err.(*net.DNSError); ok && dnsErr.IsNotFound {
			return nil
		}

		return err
	}

	if len(addrs) == 0 {
		return nil
	}

	// Attempt to extract explaination string.
	txts, err := resolver.LookupTXT(context.Background(), query)
	if err != nil || len(txts) == 0 {
		// Not significant, include addresses as reason. Usually they are
		// mapped to some predefined 'reasons' by BL.
		return ListedErr{
			Identity: domain,
			List:     cfg.Zone,
			Reason:   strings.Join(addrs, "; "),
		}
	}

	// Some BLs provide multiple reasons (meta-BLs such as Spamhaus Zen) so
	// don't mangle them by joining with "", instead join with "; ".

	return ListedErr{
		Identity: domain,
		List:     cfg.Zone,
		Reason:   strings.Join(txts, "; "),
	}
}

func checkIP(resolver dns.Resolver, cfg List, ip net.IP) error {
	ipv6 := true
	if ipv4 := ip.To4(); ipv4 != nil {
		ip = ipv4
		ipv6 = false
	}

	if ipv6 && !cfg.ClientIPv6 {
		return nil
	}
	if !ipv6 && !cfg.ClientIPv4 {
		return nil
	}

	query := queryString(ip) + "." + cfg.Zone

	addrs, err := resolver.LookupHost(context.Background(), query)
	if err != nil {
		if dnsErr, ok := err.(*net.DNSError); ok && dnsErr.IsNotFound {
			return nil
		}

		return err
	}

	if len(addrs) == 0 {
		return nil
	}

	// Attempt to extract explaination string.
	txts, err := resolver.LookupTXT(context.Background(), query)
	if err != nil || len(txts) == 0 {
		// Not significant, include addresses as reason. Usually they are
		// mapped to some predefined 'reasons' by BL.
		return ListedErr{
			Identity: ip.String(),
			List:     cfg.Zone,
			Reason:   strings.Join(addrs, "; "),
		}
	}

	// Some BLs provide multiple reasons (meta-BLs such as Spamhaus Zen) so
	// don't mangle them by joining with "", instead join with "; ".

	return ListedErr{
		Identity: ip.String(),
		List:     cfg.Zone,
		Reason:   strings.Join(txts, "; "),
	}
}

func queryString(ip net.IP) string {
	ipv6 := true
	if ipv4 := ip.To4(); ipv4 != nil {
		ip = ipv4
		ipv6 = false
	}

	res := strings.Builder{}
	if ipv6 {
		res.Grow(63) // 0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0
	} else {
		res.Grow(15) // 000.000.000.000
	}

	for i := len(ip) - 1; i >= 0; i-- {
		octet := ip[i]

		if ipv6 {
			// X.X
			res.WriteString(strconv.FormatInt(int64(octet&0xf), 16))
			res.WriteRune('.')
			res.WriteString(strconv.FormatInt(int64((octet&0xf0)>>4), 16))
		} else {
			// X
			res.WriteString(strconv.Itoa(int(octet)))
		}

		if i != 0 {
			res.WriteRune('.')
		}
	}
	return res.String()
}

// mangleErr adds smtp_* fields to DNSBL check error that mask
// details about used DNSBL.
func mangleErr(err error) error {
	_, ok := err.(ListedErr)
	if ok {
		// ListenErr is already safe due to smtp_* fields.
		return err
	}

	smtpCode := 554
	smtpEnchCode := exterrors.EnhancedCode{5, 7, 0}
	if exterrors.IsTemporary(err) {
		smtpCode = 451
		smtpEnchCode = exterrors.EnhancedCode{4, 7, 0}
	}

	return exterrors.WithFields(err, map[string]interface{}{
		"check":         "dnsbl",
		"reason":        err.Error(),
		"smtp_code":     smtpCode,
		"smtp_enchcode": smtpEnchCode,
		"smtp_msg":      "Internal error during policy check",
	})
}
