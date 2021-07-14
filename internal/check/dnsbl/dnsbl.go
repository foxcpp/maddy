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
	"errors"
	"net"
	"runtime/trace"
	"strings"
	"sync"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/framework/address"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/dns"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/target"
	"golang.org/x/sync/errgroup"
)

type List struct {
	Zone string

	ClientIPv4 bool
	ClientIPv6 bool

	EHLO     bool
	MAILFROM bool

	ScoreAdj  int
	Responses []net.IPNet
}

var defaultBL = List{
	ClientIPv4: true,
}

type DNSBL struct {
	instName   string
	checkEarly bool
	inlineBls  []string
	bls        []List

	quarantineThres int
	rejectThres     int

	resolver dns.Resolver
	log      log.Logger
}

func NewDNSBL(_, instName string, _, inlineArgs []string) (module.Module, error) {
	return &DNSBL{
		instName:  instName,
		inlineBls: inlineArgs,

		resolver: dns.DefaultResolver(),
		log:      log.Logger{Name: "dnsbl"},
	}, nil
}

func (bl *DNSBL) Name() string {
	return "dnsbl"
}

func (bl *DNSBL) InstanceName() string {
	return bl.instName
}

func (bl *DNSBL) Init(cfg *config.Map) error {
	cfg.Bool("debug", false, false, &bl.log.Debug)
	cfg.Bool("check_early", false, false, &bl.checkEarly)
	cfg.Int("quarantine_threshold", false, false, 1, &bl.quarantineThres)
	cfg.Int("reject_threshold", false, false, 9999, &bl.rejectThres)
	cfg.AllowUnknown()
	unknown, err := cfg.Process()
	if err != nil {
		return err
	}

	for _, inlineBl := range bl.inlineBls {
		cfg := defaultBL
		cfg.Zone = inlineBl
		go bl.testList(cfg)
		bl.bls = append(bl.bls, cfg)
	}

	for _, node := range unknown {
		if err := bl.readListCfg(node); err != nil {
			return err
		}
	}

	return nil
}

func (bl *DNSBL) readListCfg(node config.Node) error {
	var (
		listCfg      List
		responseNets []string
	)

	cfg := config.NewMap(nil, node)
	cfg.Bool("client_ipv4", false, defaultBL.ClientIPv4, &listCfg.ClientIPv4)
	cfg.Bool("client_ipv6", false, defaultBL.ClientIPv4, &listCfg.ClientIPv6)
	cfg.Bool("ehlo", false, defaultBL.EHLO, &listCfg.EHLO)
	cfg.Bool("mailfrom", false, defaultBL.EHLO, &listCfg.MAILFROM)
	cfg.Int("score", false, false, 1, &listCfg.ScoreAdj)
	cfg.StringList("responses", false, false, []string{"127.0.0.1/24"}, &responseNets)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	for _, resp := range responseNets {
		// If there is no / - it is a plain IP address, append
		// '/32'.
		if !strings.Contains(resp, "/") {
			resp += "/32"
		}

		_, ipNet, err := net.ParseCIDR(resp)
		if err != nil {
			return err
		}
		listCfg.Responses = append(listCfg.Responses, *ipNet)
	}

	for _, zone := range append([]string{node.Name}, node.Args...) {
		zoneCfg := listCfg
		zoneCfg.Zone = zone

		if listCfg.ScoreAdj < 0 {
			if zoneCfg.EHLO {
				return errors.New("dnsbl: 'ehlo' should not be used with negative score")
			}
			if zoneCfg.MAILFROM {
				return errors.New("dnsbl: 'mailfrom' should not be used with negative score")
			}
		}
		bl.bls = append(bl.bls, zoneCfg)

		// From RFC 5782 Section 7:
		// >To avoid this situation, systems that use
		// >DNSxLs SHOULD check for the test entries described in Section 5 to
		// >ensure that a domain actually has the structure of a DNSxL, and
		// >SHOULD NOT use any DNSxL domain that does not have correct test
		// >entries.
		// Sadly, however, many DNSBLs lack test records so at most we can
		// log a warning. Also, DNS is kinda slow so we do checks
		// asynchronously to prevent slowing down server start-up.
		go bl.testList(zoneCfg)
	}

	return nil
}

func (bl *DNSBL) testList(listCfg List) {
	// Check RFC 5782 Section 5 requirements.

	bl.log.DebugMsg("testing list for RFC 5782 requirements...", "list", listCfg.Zone)

	// 1. IPv4-based DNSxLs MUST contain an entry for 127.0.0.2 for testing purposes.
	if listCfg.ClientIPv4 {
		err := checkIP(context.Background(), bl.resolver, listCfg, net.IPv4(127, 0, 0, 2))
		if err == nil {
			bl.log.Msg("List does not contain a test record for 127.0.0.2", "list", listCfg.Zone)
		} else if _, ok := err.(ListedErr); !ok {
			bl.log.Error("lookup error, bailing out", err, "list", listCfg.Zone)
			return
		}

		// 2. IPv4-based DNSxLs MUST NOT contain an entry for 127.0.0.1.
		err = checkIP(context.Background(), bl.resolver, listCfg, net.IPv4(127, 0, 0, 1))
		if err != nil {
			_, ok := err.(ListedErr)
			if !ok {
				bl.log.Error("lookup error, bailing out", err, "list", listCfg.Zone)
				return
			}
			bl.log.Msg("List contains a record for 127.0.0.1", "list", listCfg.Zone)
		}
	}

	if listCfg.ClientIPv6 {
		// 1. IPv6-based DNSxLs MUST contain an entry for ::FFFF:7F00:2
		mustIP := net.ParseIP("::FFFF:7F00:2")

		err := checkIP(context.Background(), bl.resolver, listCfg, mustIP)
		if err == nil {
			bl.log.Msg("List does not contain a test record for ::FFFF:7F00:2", "list", listCfg.Zone)
		} else if _, ok := err.(ListedErr); !ok {
			bl.log.Error("lookup error, bailing out", err, "list", listCfg.Zone)
			return
		}

		// 2. IPv4-based DNSxLs MUST NOT contain an entry for ::FFFF:7F00:1
		mustNotIP := net.ParseIP("::FFFF:7F00:1")
		err = checkIP(context.Background(), bl.resolver, listCfg, mustNotIP)
		if err != nil {
			_, ok := err.(ListedErr)
			if !ok {
				bl.log.Error("lookup error, bailing out", err, "list", listCfg.Zone)
				return
			}
			bl.log.Msg("List contains a record for ::FFFF:7F00:1", "list", listCfg.Zone)
		}
	}

	if listCfg.EHLO || listCfg.MAILFROM {
		// Domain-name-based DNSxLs MUST contain an entry for the reserved
		// domain name "TEST".
		err := checkDomain(context.Background(), bl.resolver, listCfg, "test")
		if err == nil {
			bl.log.Msg("List does not contain a test record for 'test' TLD", "list", listCfg.Zone)
		} else if _, ok := err.(ListedErr); !ok {
			bl.log.Error("lookup error, bailing out", err, "list", listCfg.Zone)
			return
		}

		// ... and MUST NOT contain an entry for the reserved domain name
		// "INVALID".
		err = checkDomain(context.Background(), bl.resolver, listCfg, "invalid")
		if err != nil {
			_, ok := err.(ListedErr)
			if !ok {
				bl.log.Error("lookup error, bailing out", err, "list", listCfg.Zone)
				return
			}
			bl.log.Msg("List contains a record for 'invalid' TLD", "list", listCfg.Zone)
		}
	}
}

func (bl *DNSBL) checkList(ctx context.Context, list List, ip net.IP, ehlo, mailFrom string) error {
	if list.ClientIPv4 || list.ClientIPv6 {
		if err := checkIP(ctx, bl.resolver, list, ip); err != nil {
			return err
		}
	}

	if list.EHLO && ehlo != "" {
		// Skip IPs in EHLO.
		if strings.HasPrefix(ehlo, "[") && strings.HasSuffix(ehlo, "]") {
			return nil
		}

		if err := checkDomain(ctx, bl.resolver, list, ehlo); err != nil {
			return err
		}
	}

	if list.MAILFROM && mailFrom != "" {
		_, domain, err := address.Split(mailFrom)
		if err != nil || domain == "" {
			// Probably <postmaster> or <>, not much we can check.
			return nil
		}

		// If EHLO == domain (usually the case for small/private email servers)
		// then don't do a second lookup for the same domain.
		if list.EHLO && dns.Equal(domain, ehlo) {
			return nil
		}

		if err := checkDomain(ctx, bl.resolver, list, domain); err != nil {
			return err
		}
	}

	return nil
}

func (bl *DNSBL) checkLists(ctx context.Context, ip net.IP, ehlo, mailFrom string) module.CheckResult {
	var (
		eg = errgroup.Group{}

		// Protects variables below.
		lck      sync.Mutex
		score    int
		listedOn []string
		reasons  []string
	)

	for _, list := range bl.bls {
		list := list
		eg.Go(func() error {
			err := bl.checkList(ctx, list, ip, ehlo, mailFrom)
			if err != nil {
				listErr, listed := err.(ListedErr)
				if !listed {
					return err
				}

				lck.Lock()
				defer lck.Unlock()
				listedOn = append(listedOn, listErr.List)
				reasons = append(reasons, listErr.Reason)
				score += list.ScoreAdj
			}
			return nil
		})
	}

	err := eg.Wait()
	if err != nil {
		// Lookup error for BL, hard-fail.
		return module.CheckResult{
			Reject: true,
			Reason: &exterrors.SMTPError{
				Code:         exterrors.SMTPCode(err, 451, 554),
				EnhancedCode: exterrors.SMTPEnchCode(err, exterrors.EnhancedCode{0, 7, 0}),
				Message:      "DNS error during policy check",
				Err:          err,
				CheckName:    "dnsbl",
			},
		}
	}

	if score >= bl.rejectThres {
		return module.CheckResult{
			Reject: true,
			Reason: &exterrors.SMTPError{
				Code:         554,
				EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
				Message:      "Client identity is listed in the used DNSBL",
				Err:          err,
				CheckName:    "dnsbl",
			},
		}
	}
	if score >= bl.quarantineThres {
		return module.CheckResult{
			Quarantine: true,
			Reason: &exterrors.SMTPError{
				Code:         554,
				EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
				Message:      "Client identity is listed in the used DNSBL",
				Err:          err,
				CheckName:    "dnsbl",
			},
		}
	}

	return module.CheckResult{}
}

// CheckConnection implements module.EarlyCheck.
func (bl *DNSBL) CheckConnection(ctx context.Context, state *smtp.ConnectionState) error {
	if !bl.checkEarly {
		return nil
	}

	defer trace.StartRegion(ctx, "dnsbl/CheckConnection (Early)").End()

	ip, ok := state.RemoteAddr.(*net.TCPAddr)
	if !ok {
		bl.log.Msg("non-TCP/IP source",
			"src_addr", state.RemoteAddr,
			"src_host", state.Hostname)
		return nil
	}

	result := bl.checkLists(ctx, ip.IP, state.Hostname, "")
	if result.Reject {
		return result.Reason
	}

	return nil
}

type state struct {
	bl      *DNSBL
	msgMeta *module.MsgMetadata
	log     log.Logger
}

func (bl *DNSBL) CheckStateForMsg(ctx context.Context, msgMeta *module.MsgMetadata) (module.CheckState, error) {
	return &state{
		bl:      bl,
		msgMeta: msgMeta,
		log:     target.DeliveryLogger(bl.log, msgMeta),
	}, nil
}

func (s *state) CheckConnection(ctx context.Context) module.CheckResult {
	if s.bl.checkEarly {
		// Already checked before.
		return module.CheckResult{}
	}

	defer trace.StartRegion(ctx, "dnsbl/CheckConnection").End()

	if s.msgMeta.Conn == nil {
		s.log.Msg("locally generated message, ignoring")
		return module.CheckResult{}
	}

	ip, ok := s.msgMeta.Conn.RemoteAddr.(*net.TCPAddr)
	if !ok {
		s.log.Msg("non-TCP/IP source")
		return module.CheckResult{}
	}

	return s.bl.checkLists(ctx, ip.IP, s.msgMeta.Conn.Hostname, s.msgMeta.OriginalFrom)
}

func (*state) CheckSender(context.Context, string) module.CheckResult {
	return module.CheckResult{}
}

func (*state) CheckRcpt(context.Context, string) module.CheckResult {
	return module.CheckResult{}
}

func (*state) CheckBody(context.Context, textproto.Header, buffer.Buffer) module.CheckResult {
	return module.CheckResult{}
}

func (*state) Close() error {
	return nil
}

func init() {
	module.Register("check.dnsbl", NewDNSBL)
}
