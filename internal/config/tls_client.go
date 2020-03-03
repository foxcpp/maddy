package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"

	"github.com/foxcpp/maddy/internal/log"
)

func TLSClientBlock(m *Map, node Node) (interface{}, error) {
	cfg := tls.Config{}

	childM := NewMap(nil, node)
	var (
		tlsVersions       [2]uint16
		rootCAPaths       []string
		certPath, keyPath string
	)

	childM.StringList("root_ca", false, false, nil, &rootCAPaths)
	childM.String("cert", false, false, "", &certPath)
	childM.String("key", false, false, "", &keyPath)
	childM.Custom("protocols", false, false, func() (interface{}, error) {
		return [2]uint16{0, 0}, nil
	}, TLSVersionsDirective, &tlsVersions)
	childM.Custom("ciphers", false, false, func() (interface{}, error) {
		return nil, nil
	}, TLSCiphersDirective, &cfg.CipherSuites)
	childM.Custom("curves", false, false, func() (interface{}, error) {
		return nil, nil
	}, TLSCurvesDirective, &cfg.CurvePreferences)

	if _, err := m.Process(); err != nil {
		return nil, err
	}

	if len(rootCAPaths) != 0 {
		pool := x509.NewCertPool()
		for _, path := range rootCAPaths {
			blob, err := ioutil.ReadFile(path)
			if err != nil {
				return nil, err
			}
			if !pool.AppendCertsFromPEM(blob) {
				return nil, fmt.Errorf("no certificates was loaded from %s", path)
			}
		}
		cfg.RootCAs = pool
	}

	if certPath != "" || keyPath == "" {
		keypair, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return nil, err
		}
		log.Debugf("using client keypair %s/%s", certPath, keyPath)
		cfg.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return &keypair, nil
		}
	}

	cfg.MinVersion = tlsVersions[0]
	cfg.MaxVersion = tlsVersions[1]
	log.Debugf("tls: min version: %x, max version: %x", tlsVersions[0], tlsVersions[1])

	return &cfg, nil
}
