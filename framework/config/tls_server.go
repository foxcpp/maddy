package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"os"
	"sync"
	"time"

	"github.com/foxcpp/maddy/framework/hooks"
	"github.com/foxcpp/maddy/framework/log"
)

type TLSConfig struct {
	initCfg Node

	l   sync.Mutex
	cfg *tls.Config
}

func (cfg *TLSConfig) Get() *tls.Config {
	cfg.l.Lock()
	defer cfg.l.Unlock()
	if cfg.cfg == nil {
		return nil
	}
	return cfg.cfg.Clone()
}

func (cfg *TLSConfig) read(m *Map, node Node, generateSelfSig bool) error {
	cfg.l.Lock()
	defer cfg.l.Unlock()

	switch len(node.Args) {
	case 1:
		switch node.Args[0] {
		case "off":
			cfg.cfg = nil
			return nil
		case "self_signed":
			if !generateSelfSig {
				return nil
			}

			tlsCfg := &tls.Config{
				MinVersion: tls.VersionTLS10,
				MaxVersion: tls.VersionTLS13,
			}
			if err := makeSelfSignedCert(tlsCfg); err != nil {
				return err
			}
			log.Println("tls: using self-signed certificate, this is not secure!")
			cfg.cfg = tlsCfg
			return nil
		default:
			log.Println(node.Name, node.Args)
			return NodeErr(node, "unexpected argument (%s), want 'off' or 'self_signed'", node.Args[0])
		}
	case 2:
		tlsCfg, err := readTLSBlock(m, node)
		if err != nil {
			return err
		}
		cfg.cfg = tlsCfg
		return nil
	default:
		return NodeErr(node, "expected 1 or 2 arguments")
	}
}

// TLSDirective reads the TLS configuration and adds the reload handler to
// reread certificates on SIGUSR2.
//
// The returned value is *tls.TLSConfig with GetConfigForClient set.
// If the 'tls off' is used, returned value is nil.
func TLSDirective(m *Map, node Node) (interface{}, error) {
	cfg := TLSConfig{
		initCfg: node,
	}
	if err := cfg.read(m, node, true); err != nil {
		return nil, err
	}

	hooks.AddHook(hooks.EventReload, func() {
		log.Debugln("tls: reloading certificates")
		if err := cfg.read(NewMap(nil, cfg.initCfg), cfg.initCfg, false); err != nil {
			log.DefaultLogger.Error("tls: failed to load new certs", err)
		}
	})
	go func() {
		t := time.NewTicker(1 * time.Minute)
		for range t.C {
			log.Debugln("tls: reloading certificates")
			if err := cfg.read(NewMap(nil, cfg.initCfg), cfg.initCfg, false); err != nil {
				log.DefaultLogger.Error("tls: failed to load new certs", err)
			}
		}
	}()

	// Return nil so callers can check whether TLS is enabled easier.
	if cfg.cfg == nil {
		return nil, nil
	}

	return &tls.Config{
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			return cfg.Get(), nil
		},
	}, nil
}

func readTLSBlock(m *Map, blockNode Node) (*tls.Config, error) {
	cfg := tls.Config{
		PreferServerCipherSuites: true,
	}

	childM := NewMap(nil, blockNode)
	var tlsVersions [2]uint16

	if len(blockNode.Args) != 2 {
		return nil, NodeErr(blockNode, "two arguments required")
	}
	certPath := blockNode.Args[0]
	keyPath := blockNode.Args[1]

	childM.Custom("protocols", false, false, func() (interface{}, error) {
		return [2]uint16{0, 0}, nil
	}, TLSVersionsDirective, &tlsVersions)

	childM.Custom("ciphers", false, false, func() (interface{}, error) {
		return nil, nil
	}, TLSCiphersDirective, &cfg.CipherSuites)

	childM.Custom("curves", false, false, func() (interface{}, error) {
		return nil, nil
	}, TLSCurvesDirective, &cfg.CurvePreferences)

	if _, err := childM.Process(); err != nil {
		return nil, err
	}

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}

	log.Debugf("tls: using %s : %s", certPath, keyPath)
	cfg.Certificates = append(cfg.Certificates, cert)

	cfg.MinVersion = tlsVersions[0]
	cfg.MaxVersion = tlsVersions[1]
	log.Debugf("tls: min version: %x, max version: %x", tlsVersions[0], tlsVersions[1])

	return &cfg, nil
}

func makeSelfSignedCert(config *tls.Config) error {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(24 * time.Hour * 7)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return err
	}
	cert := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      pkix.Name{Organization: []string{"Maddy Self-Signed"}},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if ip := net.ParseIP(config.ServerName); ip != nil {
		cert.IPAddresses = append(cert.IPAddresses, ip)
	} else {
		cert.DNSNames = append(cert.DNSNames, config.ServerName)
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, cert, cert, &privKey.PublicKey, privKey)
	if err != nil {
		return err
	}

	config.Certificates = append(config.Certificates, tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  privKey,
		Leaf:        cert,
	})
	return nil
}

func init() {
	os.Setenv("GODEBUG", os.Getenv("GODEBUG")+",tls13=1")
}
