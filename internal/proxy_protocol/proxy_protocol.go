package proxy_protocol

import (
	"crypto/tls"
	"net"
	"strings"

	"github.com/c0va23/go-proxyprotocol"
	"github.com/foxcpp/maddy/framework/config"
	tls2 "github.com/foxcpp/maddy/framework/config/tls"
	"github.com/foxcpp/maddy/framework/log"
)

type ProxyProtocol struct {
	trust     []net.IPNet
	tlsConfig *tls.Config
}

func ProxyProtocolDirective(_ *config.Map, node config.Node) (interface{}, error) {
	p := ProxyProtocol{}

	childM := config.NewMap(nil, node)
	var trustList []string

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
		if !strings.Contains(trust, "/") {
			trust += "/32"
		}
		_, ipNet, err := net.ParseCIDR(trust)
		if err != nil {
			return nil, err
		}
		p.trust = append(p.trust, *ipNet)
	}

	return &p, nil
}

func NewListener(inner net.Listener, p *ProxyProtocol, logger log.Logger) net.Listener {
	var listener net.Listener

	sourceChecker := func(upstream net.Addr) (bool, error) {
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

		logger.Printf("proxy_protocol: connection from untrusted source %s", upstream)
		return false, nil
	}

	listener = proxyprotocol.NewDefaultListener(inner).
		WithLogger(proxyprotocol.LoggerFunc(func(format string, v ...interface{}) {
			logger.Debugf("proxy_protocol: "+format, v...)
		})).
		WithSourceChecker(sourceChecker)

	if p.tlsConfig != nil {
		listener = tls.NewListener(listener, p.tlsConfig)
	}

	return listener
}
