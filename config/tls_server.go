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
	"time"

	"github.com/foxcpp/maddy/log"
)

func TLSDirective(m *Map, node *Node) (interface{}, error) {
	switch len(node.Args) {
	case 1:
		switch node.Args[0] {
		case "off":
			return nil, nil
		case "self_signed":
			cfg := &tls.Config{
				MinVersion: tls.VersionTLS10,
				MaxVersion: tls.VersionTLS12,
			}
			if err := makeSelfSignedCert(cfg); err != nil {
				return nil, err
			}
			log.Debugln("tls: using self-signed certificate, this is not secure!")
			return cfg, nil
		}
	case 2:
		cfg, err := readTLSBlock(m, node)
		if err != nil {
			return nil, err
		}
		return cfg, nil
	default:
		return nil, m.MatchErr("expected 1 or 2 arguments")
	}

	return nil, nil
}

func readTLSBlock(m *Map, blockNode *Node) (*tls.Config, error) {
	cfg := tls.Config{
		PreferServerCipherSuites: true,
	}

	childM := NewMap(nil, blockNode)
	var tlsVersions [2]uint16

	if len(blockNode.Args) != 2 {
		return nil, m.MatchErr("two arguments required")
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

	log.Debugf("tls: using %s/%s", certPath, keyPath)
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
