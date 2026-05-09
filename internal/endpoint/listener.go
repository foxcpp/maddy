package endpoint

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/foxcpp/maddy/framework/config"
	tls2 "github.com/foxcpp/maddy/framework/config/tls"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/resource/netresource"
	"github.com/foxcpp/maddy/internal/proxy_protocol"
)

type Server interface {
	Serve(ln net.Listener) error
	Close() error
}

type ShutdownServer interface {
	Shutdown(ctx context.Context) error
}

type MetaListener struct {
	server Server
	log    *log.Logger
	addrs  []string

	proxyProtocol *proxy_protocol.ProxyProtocol
	TLSConfig     *tls.Config

	endpoints   []config.Endpoint
	listeners   []net.Listener
	listenersWg sync.WaitGroup
}

func NewMetaListener(l *log.Logger, addrs []string) *MetaListener {
	return &MetaListener{
		log:   l,
		addrs: addrs,
	}
}

func (l *MetaListener) AddConfig(cfg *config.Map) {
	cfg.Custom("tls", true, true, nil, tls2.TLSDirective, &l.TLSConfig)
	cfg.Custom("proxy_protocol", false, false, nil, proxy_protocol.ProxyProtocolDirective, &l.proxyProtocol)
}

func (l *MetaListener) ParseEndpoints() error {
	addresses := make([]config.Endpoint, 0, len(l.addrs))
	for _, addr := range l.addrs {
		saddr, err := config.ParseEndpoint(addr)
		if err != nil {
			return fmt.Errorf("listener: invalid address: %s", addr)
		}
		if saddr.IsTLS() && l.TLSConfig == nil {
			return errors.New("can't bind on IMAPS endpoint without TLS configuration")
		}
		addresses = append(addresses, saddr)
	}
	l.endpoints = addresses

	return nil
}

func (l *MetaListener) ListenAndServe(serv Server) error {
	l.server = serv

	for _, addr := range l.endpoints {
		var ln net.Listener
		var err error
		ln, err = netresource.Listen(addr.Network(), addr.Address())
		if err != nil {
			return fmt.Errorf("listener: %v", err)
		}
		l.log.Printf("listening on %v", addr)

		if addr.IsTLS() {
			if l.TLSConfig == nil {
				return errors.New("can't bind on IMAPS endpoint without TLS configuration")
			}
			ln = tls.NewListener(ln, l.TLSConfig)
		}

		if l.proxyProtocol != nil {
			ln = proxy_protocol.NewListener(ln, l.proxyProtocol, l.log)
		}

		l.listeners = append(l.listeners, ln)
		l.listenersWg.Add(1)
		go func() {
			defer l.listenersWg.Done()
			if err := l.server.Serve(ln); err != nil && !strings.HasSuffix(err.Error(), "use of closed network connection") {
				l.log.Printf("failed to serve %s: %s", addr, err)
			}
		}()
	}

	return nil
}

func (l *MetaListener) Close() error {
	for _, ln := range l.listeners {
		if err := ln.Close(); err != nil {
			l.log.Printf("listener: failed to close listener: %s", err)
		}
	}

	if shut, ok := l.server.(ShutdownServer); ok {
		if err := shut.Shutdown(context.Background()); err != nil && !errors.Is(err, context.Canceled) {
			l.log.Printf("graceful shutdown failed: %s", err)
		}
	}

	if err := l.server.Close(); err != nil {
		return err
	}
	l.listenersWg.Wait()
	return nil
}
