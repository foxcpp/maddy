package dnsbl

import (
	"errors"
	"net"
	"strings"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/internal/address"
	"github.com/foxcpp/maddy/internal/buffer"
	"github.com/foxcpp/maddy/internal/check"
	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/dns"
	"github.com/foxcpp/maddy/internal/exterrors"
	"github.com/foxcpp/maddy/internal/log"
	"github.com/foxcpp/maddy/internal/module"
	"github.com/foxcpp/maddy/internal/target"
	"golang.org/x/sync/errgroup"
)

type List struct {
	Zone string

	ClientIPv4 bool
	ClientIPv6 bool

	EHLO     bool
	MAILFROM bool

	Responses []net.IPNet
}

var defaultBL = List{
	ClientIPv4: true,
}

type DNSBL struct {
	instName     string
	checkEarly   bool
	listedAction check.FailAction
	inlineBls    []string
	bls          []List
	wls          []List

	resolver dns.Resolver
	log      log.Logger
}

func NewDNSBL(_, instName string, _, inlineArgs []string) (module.Module, error) {
	return &DNSBL{
		instName:  instName,
		inlineBls: inlineArgs,

		resolver: net.DefaultResolver,
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
	cfg.Custom("listed_action", false, false,
		func() (interface{}, error) {
			return check.FailAction{Reject: true}, nil
		}, check.FailActionDirective, &bl.listedAction)
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
		whitelist    bool
		responseNets []string
	)

	cfg := config.NewMap(nil, &node)
	cfg.Bool("client_ipv4", false, defaultBL.ClientIPv4, &listCfg.ClientIPv4)
	cfg.Bool("client_ipv6", false, defaultBL.ClientIPv4, &listCfg.ClientIPv6)
	cfg.Bool("ehlo", false, defaultBL.EHLO, &listCfg.EHLO)
	cfg.Bool("mailfrom", false, defaultBL.EHLO, &listCfg.MAILFROM)
	cfg.Bool("whitelist", false, false, &whitelist)
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

		if whitelist {
			if zoneCfg.EHLO {
				return errors.New("dnsbl: 'ehlo' can't be used with 'whitelist'")
			}
			if zoneCfg.MAILFROM {
				return errors.New("dnsbl: 'mailfrom' can't be used with 'whitelist'")
			}

			bl.wls = append(bl.wls, zoneCfg)
		} else {
			bl.bls = append(bl.bls, zoneCfg)
		}

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
		err := checkIP(bl.resolver, listCfg, net.IPv4(127, 0, 0, 2))
		if err == nil {
			bl.log.Msg("List does not contain a test record for 127.0.0.2", "list", listCfg.Zone)
		} else if _, ok := err.(ListedErr); !ok {
			bl.log.Error("lookup error, bailing out", err, "list", listCfg.Zone)
			return
		}

		// 2. IPv4-based DNSxLs MUST NOT contain an entry for 127.0.0.1.
		err = checkIP(bl.resolver, listCfg, net.IPv4(127, 0, 0, 1))
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

		err := checkIP(bl.resolver, listCfg, mustIP)
		if err == nil {
			bl.log.Msg("List does not contain a test record for ::FFFF:7F00:2", "list", listCfg.Zone)
		} else if _, ok := err.(ListedErr); !ok {
			bl.log.Error("lookup error, bailing out", err, "list", listCfg.Zone)
			return
		}

		// 2. IPv4-based DNSxLs MUST NOT contain an entry for 127.0.0.1.
		mustNotIP := net.ParseIP("::FFFF:7F00:1")
		err = checkIP(bl.resolver, listCfg, mustNotIP)
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
		err := checkDomain(bl.resolver, listCfg, "test")
		if err == nil {
			bl.log.Msg("List does not contain a test record for 'test' TLD", "list", listCfg.Zone)
		} else if _, ok := err.(ListedErr); !ok {
			bl.log.Error("lookup error, bailing out", err, "list", listCfg.Zone)
			return
		}

		// ... and MUST NOT contain an entry for the reserved domain name
		// "INVALID".
		err = checkDomain(bl.resolver, listCfg, "invalid")
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

func (bl *DNSBL) checkList(list List, ip net.IP, ehlo, mailFrom string) error {
	if list.ClientIPv4 || list.ClientIPv6 {
		if err := checkIP(bl.resolver, list, ip); err != nil {
			return err
		}
	}

	if list.EHLO && ehlo != "" {
		// Skip IPs in EHLO.
		if strings.HasPrefix(ehlo, "[") && strings.HasSuffix(ehlo, "]") {
			return nil
		}

		if err := checkDomain(bl.resolver, list, ehlo); err != nil {
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

		if err := checkDomain(bl.resolver, list, domain); err != nil {
			return err
		}
	}

	return nil
}

func (bl *DNSBL) checkLists(ip net.IP, ehlo, mailFrom string) error {
	eg := errgroup.Group{}

	for _, list := range bl.bls {
		list := list
		eg.Go(func() error {
			return bl.checkList(list, ip, ehlo, mailFrom)
		})
	}

	err := eg.Wait()
	_, listed := err.(ListedErr)
	if !listed {
		// Lookup error for BL, hard-fail.
		return err
	}
	if len(bl.wls) == 0 {
		// No whitelists, hence not worth checking.
		return err
	}

	eg = errgroup.Group{}

	// Check configured whitelists.
	for _, list := range bl.wls {
		list := list
		eg.Go(func() error {
			return bl.checkList(list, ip, ehlo, mailFrom)
		})
	}

	wlerr := eg.Wait()
	if wlErr, listedWl := wlerr.(ListedErr); listedWl {
		// Listed on WL, override the BL result to 'neutral'.
		bl.log.Msg("WL overrides BL listing", "list", wlErr.List, "listed_identity", wlErr.Identity)
		return nil
	}

	// Lookup error for WL, hard-fail.
	return wlerr
}

// CheckConnection implements module.EarlyCheck.
func (bl *DNSBL) CheckConnection(state *smtp.ConnectionState) error {
	if !bl.checkEarly {
		return nil
	}

	ip, ok := state.RemoteAddr.(*net.TCPAddr)
	if !ok {
		bl.log.Msg("non-TCP/IP source",
			"src_addr", state.RemoteAddr,
			"src_host", state.Hostname)
		return nil
	}

	if err := bl.checkLists(ip.IP, state.Hostname, ""); err != nil {
		return exterrors.WithFields(err, map[string]interface{}{"check": "dnsbl"})
	}

	return nil
}

type state struct {
	bl      *DNSBL
	msgMeta *module.MsgMetadata
	log     log.Logger
}

func (bl *DNSBL) CheckStateForMsg(msgMeta *module.MsgMetadata) (module.CheckState, error) {
	return &state{
		bl:      bl,
		msgMeta: msgMeta,
		log:     target.DeliveryLogger(bl.log, msgMeta),
	}, nil
}

func (s *state) CheckConnection() module.CheckResult {
	if s.bl.checkEarly {
		// Already checked before.
		return module.CheckResult{}
	}

	if s.msgMeta.Conn == nil {
		s.log.Msg("locally generated message, ignoring")
		return module.CheckResult{}
	}

	ip, ok := s.msgMeta.Conn.RemoteAddr.(*net.TCPAddr)
	if !ok {
		s.log.Msg("non-TCP/IP source")
		return module.CheckResult{}
	}

	if err := s.bl.checkLists(ip.IP, s.msgMeta.Conn.Hostname, s.msgMeta.OriginalFrom); err != nil {
		// TODO: Support per-list actions?
		return s.bl.listedAction.Apply(module.CheckResult{
			Reason: exterrors.WithFields(err, map[string]interface{}{"check": "dnsbl"}),
		})
	}

	s.log.DebugMsg("ok")

	return module.CheckResult{}
}

func (*state) CheckSender(string) module.CheckResult {
	return module.CheckResult{}
}

func (*state) CheckRcpt(string) module.CheckResult {
	return module.CheckResult{}
}

func (*state) CheckBody(textproto.Header, buffer.Buffer) module.CheckResult {
	return module.CheckResult{}
}

func (*state) Close() error {
	return nil
}

func init() {
	module.Register("dnsbl", NewDNSBL)
}
