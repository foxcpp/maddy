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

package dnsbl

import (
	"context"
	"net"
	"strconv"
	"strings"

	"github.com/foxcpp/maddy/framework/dns"
	"github.com/foxcpp/maddy/framework/exterrors"
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

func checkDomain(ctx context.Context, resolver dns.Resolver, cfg List, domain string) error {
	query := domain + "." + cfg.Zone

	addrs, err := resolver.LookupHost(ctx, query)
	if err != nil {
		if dnsErr, ok := err.(*net.DNSError); ok && dnsErr.IsNotFound {
			return nil
		}

		return err
	}

	if len(addrs) == 0 {
		return nil
	}

	// Attempt to extract explanation string.
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

func checkIP(ctx context.Context, resolver dns.Resolver, cfg List, ip net.IP) error {
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

	addrs, err := resolver.LookupIPAddr(ctx, query)
	if err != nil {
		if dnsErr, ok := err.(*net.DNSError); ok && dnsErr.IsNotFound {
			return nil
		}

		return err
	}

	filteredAddrs := make([]net.IPAddr, 0, len(addrs))
addrsLoop:
	for _, addr := range addrs {
		// No responses whitelist configured - permit all.
		if len(cfg.Responses) == 0 {
			filteredAddrs = append(filteredAddrs, addr)
			continue
		}

		for _, respNet := range cfg.Responses {
			if respNet.Contains(addr.IP) {
				filteredAddrs = append(filteredAddrs, addr)
				continue addrsLoop
			}
		}
	}

	if len(filteredAddrs) == 0 {
		return nil
	}

	// Attempt to extract explanation string.
	txts, err := resolver.LookupTXT(ctx, query)
	if err != nil || len(txts) == 0 {
		// Not significant, include addresses as reason. Usually they are
		// mapped to some predefined 'reasons' by BL.

		reasonParts := make([]string, 0, len(filteredAddrs))
		for _, addr := range filteredAddrs {
			reasonParts = append(reasonParts, addr.IP.String())
		}

		return ListedErr{
			Identity: ip.String(),
			List:     cfg.Zone,
			Reason:   strings.Join(reasonParts, "; "),
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
