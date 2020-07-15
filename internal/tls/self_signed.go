package tls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"time"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
)

type SelfSignedLoader struct {
	instName    string
	serverNames []string

	cert tls.Certificate
}

func NewSelfSignedLoader(_, instName string, _, inlineArgs []string) (module.Module, error) {
	return &SelfSignedLoader{
		instName:    instName,
		serverNames: inlineArgs,
	}, nil
}

func (f *SelfSignedLoader) Init(cfg *config.Map) error {
	if _, err := cfg.Process(); err != nil {
		return err
	}

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

	for _, name := range f.serverNames {
		if ip := net.ParseIP(name); ip != nil {
			cert.IPAddresses = append(cert.IPAddresses, ip)
		} else {
			cert.DNSNames = append(cert.DNSNames, name)
		}
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, cert, cert, &privKey.PublicKey, privKey)
	if err != nil {
		return err
	}

	f.cert = tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  privKey,
		Leaf:        cert,
	}
	return nil
}

func (f *SelfSignedLoader) Name() string {
	return "tls.loader.self_signed"
}

func (f *SelfSignedLoader) InstanceName() string {
	return f.instName
}

func (f *SelfSignedLoader) LoadCerts() ([]tls.Certificate, error) {
	return []tls.Certificate{f.cert}, nil
}

func init() {
	module.Register("tls.loader.self_signed", NewSelfSignedLoader)
}
