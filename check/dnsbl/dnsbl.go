package dnsbl

import (
	"net"
	"strings"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/address"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/check"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/dns"
	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/module"
	"github.com/foxcpp/maddy/target"
	"golang.org/x/sync/errgroup"
)

type BL struct {
	Zone string

	ClientIPv4 bool
	ClientIPv6 bool

	EHLO     bool
	MAILFROM bool
}

var defaultBL = BL{
	ClientIPv4: true,
}

type DNSBL struct {
	instName     string
	checkEarly   bool
	listedAction check.FailAction
	inlineBls    []string
	bls          []BL

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
	unmatched, err := cfg.Process()
	if err != nil {
		return err
	}

	for _, inlineBl := range bl.inlineBls {
		cfg := defaultBL
		cfg.Zone = inlineBl
		go bl.testBL(cfg)
		bl.bls = append(bl.bls, cfg)
	}

	for _, node := range unmatched {
		if err := bl.readBLCfg(node); err != nil {
			return err
		}
	}

	return nil
}

func (bl *DNSBL) readBLCfg(node config.Node) error {
	var blCfg BL

	cfg := config.NewMap(nil, &node)
	cfg.Bool("client_ipv4", false, defaultBL.ClientIPv4, &blCfg.ClientIPv4)
	cfg.Bool("client_ipv6", false, defaultBL.ClientIPv4, &blCfg.ClientIPv6)
	cfg.Bool("ehlo", false, defaultBL.EHLO, &blCfg.EHLO)
	cfg.Bool("mailfrom", false, defaultBL.EHLO, &blCfg.MAILFROM)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	for _, zone := range append([]string{node.Name}, node.Args...) {
		// From RFC 5782 Section 7:
		// >To avoid this situation, systems that use
		// >DNSxLs SHOULD check for the test entries described in Section 5 to
		// >ensure that a domain actually has the structure of a DNSxL, and
		// >SHOULD NOT use any DNSxL domain that does not have correct test
		// >entries.
		// Sadly, however, many DNSBLs lack test records so at most we can
		// log a warning. Also, DNS is kinda slow so we do checks
		// asynchronously to prevent slowing down server start-up.

		zoneCfg := blCfg
		zoneCfg.Zone = zone
		go bl.testBL(zoneCfg)
		bl.bls = append(bl.bls, zoneCfg)
	}

	return nil
}

func (bl *DNSBL) testBL(listCfg BL) {
	// Check RFC 5782 Section 5 requirements.

	bl.log.DebugMsg("testing BL for RFC 5782 requirements...", "dnsbl", listCfg.Zone)

	// 1. IPv4-based DNSxLs MUST contain an entry for 127.0.0.2 for testing purposes.
	if listCfg.ClientIPv4 {
		err := checkIP(bl.resolver, listCfg, net.IPv4(127, 0, 0, 2))
		if err == nil {
			bl.log.Msg("BL does not contain a test record for 127.0.0.2", "dnsbl", listCfg.Zone)
		} else if _, ok := err.(ListedErr); !ok {
			bl.log.Error("lookup error, bailing out", err, "dnsbl", listCfg.Zone)
			return
		}

		// 2. IPv4-based DNSxLs MUST NOT contain an entry for 127.0.0.1.
		err = checkIP(bl.resolver, listCfg, net.IPv4(127, 0, 0, 1))
		if err != nil {
			_, ok := err.(ListedErr)
			if !ok {
				bl.log.Error("lookup error, bailing out", err, "dnsbl", listCfg.Zone)
				return
			}
			bl.log.Msg("BL contains a record for 127.0.0.1", "dnsbl", listCfg.Zone)
		}
	}

	if listCfg.ClientIPv6 {
		// 1. IPv6-based DNSxLs MUST contain an entry for ::FFFF:7F00:2
		mustIP := net.ParseIP("::FFFF:7F00:2")

		err := checkIP(bl.resolver, listCfg, mustIP)
		if err == nil {
			bl.log.Msg("BL does not contain a test record for ::FFFF:7F00:2", "dnsbl", listCfg.Zone)
		} else if _, ok := err.(ListedErr); !ok {
			bl.log.Error("lookup error, bailing out", err, "dnsbl", listCfg.Zone)
			return
		}

		// 2. IPv4-based DNSxLs MUST NOT contain an entry for 127.0.0.1.
		mustNotIP := net.ParseIP("::FFFF:7F00:1")
		err = checkIP(bl.resolver, listCfg, mustNotIP)
		if err != nil {
			_, ok := err.(ListedErr)
			if !ok {
				bl.log.Error("lookup error, bailing out", err, "dnsbl", listCfg.Zone)
				return
			}
			bl.log.Msg("BL contains a record for ::FFFF:7F00:1", "dnsbl", listCfg.Zone)
		}
	}

	if listCfg.EHLO || listCfg.MAILFROM {
		// Domain-name-based DNSxLs MUST contain an entry for the reserved
		// domain name "TEST".
		err := checkDomain(bl.resolver, listCfg, "test")
		if err == nil {
			bl.log.Msg("BL does not contain a test record for 'test' TLD", "dnsbl", listCfg.Zone)
		} else if _, ok := err.(ListedErr); !ok {
			bl.log.Error("lookup error, bailing out", err, "dnsbl", listCfg.Zone)
			return
		}

		// ... and MUST NOT contain an entry for the reserved domain name
		// "INVALID".
		err = checkDomain(bl.resolver, listCfg, "invalid")
		if err != nil {
			_, ok := err.(ListedErr)
			if !ok {
				bl.log.Error("lookup error, bailing out", err, "dnsbl", listCfg.Zone)
				return
			}
			bl.log.Msg("BL contains a record for 'invalid' TLD", "dnsbl", listCfg.Zone)
		}
	}
}

func (bl *DNSBL) checkPreBody(ip net.IP, ehlo, mailFrom string) error {
	eg := errgroup.Group{}

	for _, list := range bl.bls {
		list := list
		eg.Go(func() error {
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

				if err := checkDomain(bl.resolver, list, domain); err != nil {
					return err
				}
			}

			return nil

		})
	}

	// TODO: Whitelists support.
	// ... if there is error and it is a ListenErr, then check whitelists
	// for whether it is whitelisted.

	return eg.Wait()
}

// CheckConnection implements module.EarlyCheck.
func (bl *DNSBL) CheckConnection(state *smtp.ConnectionState) error {
	ip, ok := state.RemoteAddr.(*net.TCPAddr)
	if !ok {
		bl.log.Msg("non-TCP/IP source",
			"src_addr", state.RemoteAddr,
			"src_host", state.Hostname)
		return nil
	}

	if err := bl.checkPreBody(ip.IP, state.Hostname, ""); err != nil {
		return mangleErr(err)
	}

	return nil
}

type state struct {
	bl      *DNSBL
	msgMeta *module.MsgMetadata
	log     log.Logger
}

func (bl *DNSBL) CheckStateForMsg(msgMeta *module.MsgMetadata) (module.CheckState, error) {
	return state{
		bl:      bl,
		msgMeta: msgMeta,
		log:     target.DeliveryLogger(bl.log, msgMeta),
	}, nil
}

func (s state) CheckConnection() module.CheckResult {
	ip, ok := s.msgMeta.SrcAddr.(*net.TCPAddr)
	if !ok {
		s.log.Msg("non-TCP/IP source")
		return module.CheckResult{}
	}

	if err := s.bl.checkPreBody(ip.IP, s.msgMeta.SrcHostname, s.msgMeta.OriginalFrom); err != nil {
		// TODO: Support per-list actions?
		return s.bl.listedAction.Apply(module.CheckResult{
			Reason: mangleErr(err),
		})
	}

	s.log.DebugMsg("ok")

	return module.CheckResult{}
}

func (state) CheckSender(string) module.CheckResult {
	return module.CheckResult{}
}

func (state) CheckRcpt(string) module.CheckResult {
	return module.CheckResult{}
}

func (state) CheckBody(textproto.Header, buffer.Buffer) module.CheckResult {
	return module.CheckResult{}
}

func (state) Close() error {
	return nil
}

func init() {
	module.Register("dnsbl", NewDNSBL)
}
