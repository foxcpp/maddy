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
	case 0:
		cfg, err := readTlsBlock(node)
		if err != nil {
			return nil, err
		}
		return cfg, nil
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
			return &cfg, nil
		}
	case 2:
		cfg := tls.Config{}
		cert, err := tls.LoadX509KeyPair(node.Args[0], node.Args[1])
		if err != nil {
			return nil, err
		}

		log.Debugf("tls: using %s/%s", node.Args[0], node.Args[1])
		cfg.Certificates = append(cfg.Certificates, cert)

		return &cfg, nil
	default:
		return nil, m.MatchErr("expected 1 or 2 arguments")
	}

	return nil, nil
}

var strVersionsMap = map[string]uint16{
	"SSL3.0": tls.VersionSSL30,
	"TLS1.0": tls.VersionTLS10,
	"TLS1.1": tls.VersionTLS11,
	"TLS1.2": tls.VersionTLS12,
	"TLS1.3": tls.VersionTLS13,
	"":       0, // use crypto/tls defaults if value is not specified
}

var strCiphersMap = map[string]uint16{
	// TLS 1.0 - 1.2 cipher suites.
	"TLS_RSA_WITH_RC4_128_SHA":                0x0005,
	"TLS_RSA_WITH_3DES_EDE_CBC_SHA":           0x000a,
	"TLS_RSA_WITH_AES_128_CBC_SHA":            0x002f,
	"TLS_RSA_WITH_AES_256_CBC_SHA":            0x0035,
	"TLS_RSA_WITH_AES_128_CBC_SHA256":         0x003c,
	"TLS_RSA_WITH_AES_128_GCM_SHA256":         0x009c,
	"TLS_RSA_WITH_AES_256_GCM_SHA384":         0x009d,
	"TLS_ECDHE_ECDSA_WITH_RC4_128_SHA":        0xc007,
	"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA":    0xc009,
	"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA":    0xc00a,
	"TLS_ECDHE_RSA_WITH_RC4_128_SHA":          0xc011,
	"TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA":     0xc012,
	"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA":      0xc013,
	"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA":      0xc014,
	"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256": 0xc023,
	"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256":   0xc027,
	"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256":   0xc02f,
	"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256": 0xc02b,
	"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384":   0xc030,
	"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384": 0xc02c,
	"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305":    0xcca8,
	"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305":  0xcca9,

	// TLS 1.3 cipher suites.
	"TLS_AES_128_GCM_SHA256":       0x1301,
	"TLS_AES_256_GCM_SHA384":       0x1302,
	"TLS_CHACHA20_POLY1305_SHA256": 0x1303,
}

var strCurvesMap = map[string]tls.CurveID{
	"P256":   23,
	"P384":   24,
	"P521":   25,
	"X25519": 29,
}

func readTlsBlock(blockNode *config.Node) (*tls.Config, error) {
	cfg := tls.Config{
		PreferServerCipherSuites: true,
	}

	m := config.NewMap(nil, blockNode)
	var minVersionStr, maxVersionStr string
	m.String("min_version", false, false, "", &minVersionStr)
	m.String("max_version", false, false, "", &maxVersionStr)

	m.Custom("ciphers", false, false, func() (interface{}, error) {
		return nil, nil
	}, func(_ *config.Map, node *config.Node) (interface{}, error) {
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
	}, func(_ *config.Map, node *config.Node) (interface{}, error) {
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

	m.AllowUnknown()
	unmatched, err := m.Process()
	if err != nil {
		return nil, err
	}

	var ok bool
	cfg.MinVersion, ok = strVersionsMap[minVersionStr]
	if !ok {
		return nil, m.MatchErr("unknown TLS version: %s", minVersionStr)
	}
	cfg.MaxVersion, ok = strVersionsMap[maxVersionStr]
	if !ok {
		return nil, m.MatchErr("unknown TLS version: %s", maxVersionStr)
	}
	log.Debugf("tls: min version: %s (%x), max version: %s (%x)", minVersionStr, cfg.MinVersion, maxVersionStr, cfg.MaxVersion)

	for _, subnode := range unmatched {
		if subnode.Name == "self_signed" {
			if err := makeSelfSignedCert(&cfg); err != nil {
				return nil, m.MatchErr("%v", err)
			}
			log.Debugf("tls: using self-signed certificate, this is not secure!")
			continue
		}
		if len(subnode.Args) != 1 {
			return nil, m.MatchErr("expected 2 (cert & key) values, got %d", len(subnode.Args)-1)
		}

		cert, err := tls.LoadX509KeyPair(subnode.Name, subnode.Args[0])
		if err != nil {
			return nil, m.MatchErr("%v", err)
		}
		cfg.Certificates = append(cfg.Certificates, cert)
		log.Debugf("tls: using %s/%s", subnode.Name, subnode.Args[0])
	}

	if len(cfg.Certificates) == 0 {
		return nil, m.MatchErr("at least one certificate required")
	}

	cfg.BuildNameToCertificate()
	log.Debugf("NameToCert:")
	for k, v := range cfg.NameToCertificate {
		// last certificate is usually us, and all prev. are intermediates.
		cert, err := x509.ParseCertificate(v.Certificate[len(v.Certificate)-1])
		if err != nil {
			return nil, err
		}
		log.Debugf("%v => %x", k, cert.SerialNumber.Bytes()) // print hex-encoded SerialNumber
	}

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
