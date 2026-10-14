package proxy_protocol

import (
	"crypto/tls"
	"errors"
	"net"
	"strings"

	"github.com/c0va23/go-proxyprotocol"
	"github.com/foxcpp/maddy/framework/config"
	tls2 "github.com/foxcpp/maddy/framework/config/tls"
	"github.com/foxcpp/maddy/framework/log"
)

type ProxyProtocol struct {
	trust     []net.IPNet
	trustAll  bool
	tlsConfig *tls.Config
}

func ProxyProtocolDirective(_ *config.Map, node config.Node) (interface{}, error) {
	p := ProxyProtocol{}
	var trustList []string

	childM := config.NewMap(nil, node)

	childM.StringList("trust", false, false, nil, &trustList)
	childM.Custom("tls", true, false, nil, tls2.TLSDirective, &p.tlsConfig)

	if _, err := childM.Process(); err != nil {
		return nil, err
	}

	if len(node.Args) > 0 {
		if trustList == nil {
			trustList = make([]string, 0)
		}
		trustList = append(trustList, node.Args...)
	}

	for _, trust := range trustList {
		if strings.EqualFold(trust, "all") {
			p.trustAll = true
			continue
		}
		if !strings.Contains(trust, "/") {
			trust += "/32"
		}
		_, ipNet, err := net.ParseCIDR(trust)
		if err != nil {
			return nil, err
		}
		p.trust = append(p.trust, *ipNet)
	}

	if len(p.trust) == 0 && !p.trustAll {
		return nil, errors.New("proxy_protocol requires explicit 'trust all' to allow any client to use PROXY")
	}

	return &p, nil
}

func NewListener(inner net.Listener, p *ProxyProtocol, logger *log.Logger) net.Listener {
	var listener net.Listener

	sourceChecker := func(upstream net.Addr) (bool, error) {
		if p.trustAll {
			return true, nil
		}

		if tcpAddr, ok := upstream.(*net.TCPAddr); ok {
			if len(p.trust) == 0 {
				return true, nil
			}
			for _, trusted := range p.trust {
				if trusted.Contains(tcpAddr.IP) {
					return true, nil
				}
			}
		} else if _, ok := upstream.(*net.UnixAddr); ok {
			// UNIX local socket connection, always trusted
			return true, nil
		}

		logger.Printf("connection from untrusted source %s", upstream)
		return false, nil
	}

	if p.trustAll {
		logger.Msg("WARNING: proxy_protocol allows any IP to use PROXY - this is potentially unsafe")
	}

	proxyListener := proxyprotocol.NewDefaultListener(inner).
		WithLogger(proxyprotocol.LoggerFunc(logger.Debugf)).
		WithSourceChecker(sourceChecker)
	listener = &proxyListener

	if p.tlsConfig != nil {
		listener = tls.NewListener(listener, p.tlsConfig)
	}

	return listener
}
