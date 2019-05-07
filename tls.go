package maddy

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

	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/log"
)

func tlsDirective(m *config.Map, node *config.Node) (interface{}, error) {
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
		cfg, err := readTLSBlock(node)
		if err != nil {
			return nil, err
		}
		return cfg, nil
	default:
		return nil, m.MatchErr("expected 1 or 2 arguments")
	}

	return nil, nil
}

var strVersionsMap = map[string]uint16{
	"tls1.0": tls.VersionTLS10,
	"tls1.1": tls.VersionTLS11,
	"tls1.2": tls.VersionTLS12,
	"tls1.3": tls.VersionTLS13,
	"":       0, // use crypto/tls defaults if value is not specified
}

var strCiphersMap = map[string]uint16{
	// TLS 1.0 - 1.2 cipher suites.
	"RSA-WITH-RC4128-SHA":                tls.TLS_RSA_WITH_RC4_128_SHA,
	"RSA-WITH-3DES-EDE-CBC-SHA":          tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	"RSA-WITH-AES128-CBC-SHA":            tls.TLS_RSA_WITH_AES_128_CBC_SHA,
	"RSA-WITH-AES256-CBC-SHA":            tls.TLS_RSA_WITH_AES_256_CBC_SHA,
	"RSA-WITH-AES128-CBC-SHA256":         tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
	"RSA-WITH-AES128-GCM-SHA256":         tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	"RSA-WITH-AES256-GCM-SHA384":         tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	"ECDHE-ECDSA-WITH-RC4128-SHA":        tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA,
	"ECDHE-ECDSA-WITH-AES128-CBC-SHA":    tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
	"ECDHE-ECDSA-WITH-AES256-CBC-SHA":    tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
	"ECDHE-RSA-WITH-RC4128-SHA":          tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
	"ECDHE-RSA-WITH-3DES-EDE-CBC-SHA":    tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
	"ECDHE-RSA-WITH-AES128-CBC-SHA":      tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	"ECDHE-RSA-WITH-AES256-CBC-SHA":      tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	"ECDHE-ECDSA-WITH-AES128-CBC-SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
	"ECDHE-RSA-WITH-AES128-CBC-SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
	"ECDHE-RSA-WITH-AES128-GCM-SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	"ECDHE-ECDSA-WITH-AES128-GCM-SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	"ECDHE-RSA-WITH-AES256-GCM-SHA384":   tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	"ECDHE-ECDSA-WITH-AES256-GCM-SHA384": tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	"ECDHE-RSA-WITH-CHACHA20-POLY1305":   tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	"ECDHE-ECDSA-WITH-CHACHA20-POLY1305": tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
}

var strCurvesMap = map[string]tls.CurveID{
	"p256":   tls.CurveP256,
	"p384":   tls.CurveP384,
	"p521":   tls.CurveP521,
	"X25519": tls.X25519,
}

func readTLSBlock(blockNode *config.Node) (*tls.Config, error) {
	cfg := tls.Config{
		PreferServerCipherSuites: true,
	}

	m := config.NewMap(nil, blockNode)
	var tlsVersions [2]uint16

	if len(blockNode.Args) != 2 {
		return nil, config.NodeErr(blockNode, "two arguments required")
	}
	certPath := blockNode.Args[0]
	keyPath := blockNode.Args[1]

	m.Custom("protocols", false, false, func() (interface{}, error) {
		return [2]uint16{0, 0}, nil
	}, func(m *config.Map, node *config.Node) (interface{}, error) {
		switch len(node.Args) {
		case 1:
			value, ok := strVersionsMap[node.Args[0]]
			if !ok {
				return nil, m.MatchErr("invalid TLS version value: %s", node.Args[0])
			}
			return [2]uint16{value, value}, nil
		case 2:
			minValue, ok := strVersionsMap[node.Args[0]]
			if !ok {
				return nil, m.MatchErr("invalid TLS version value: %s", node.Args[0])
			}
			maxValue, ok := strVersionsMap[node.Args[1]]
			if !ok {
				return nil, m.MatchErr("invalid TLS version value: %s", node.Args[1])
			}
			return [2]uint16{minValue, maxValue}, nil
		default:
			return nil, m.MatchErr("expected 1 or 2 arguments")
		}
	}, &tlsVersions)

	m.Custom("ciphers", false, false, func() (interface{}, error) {
		return nil, nil
	}, func(m *config.Map, node *config.Node) (interface{}, error) {
		if len(node.Args) == 0 {
			return nil, m.MatchErr("expected at least 1 argument, got 0")
		}

		res := make([]uint16, 0, len(node.Args))
		for _, arg := range node.Args {
			cipherId, ok := strCiphersMap[arg]
			if !ok {
				return nil, m.MatchErr("unknown cipher: %s", arg)
			}
			res = append(res, cipherId)
		}
		log.Debugln("tls: using non-default cipherset:", node.Args)
		return res, nil
	}, &cfg.CipherSuites)

	m.Custom("curves", false, false, func() (interface{}, error) {
		return nil, nil
	}, func(m *config.Map, node *config.Node) (interface{}, error) {
		if len(node.Args) == 0 {
			return nil, m.MatchErr("expected at least 1 argument, got 0")
		}

		res := make([]tls.CurveID, 0, len(node.Args))
		for _, arg := range node.Args {
			curveId, ok := strCurvesMap[arg]
			if !ok {
				return nil, m.MatchErr("unknown curve: %s", arg)
			}
			res = append(res, curveId)
		}
		log.Debugln("tls: using non-default curve perferences:", node.Args)
		return res, nil
	}, &cfg.CurvePreferences)

	if _, err := m.Process(); err != nil {
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
