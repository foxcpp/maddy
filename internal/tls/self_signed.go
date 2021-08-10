/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

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

func (f *SelfSignedLoader) ConfigureTLS(c *tls.Config) error {
	c.Certificates = []tls.Certificate{f.cert}
	return nil
}

func init() {
	var _ module.TLSLoader = &SelfSignedLoader{}
	module.Register("tls.loader.self_signed", NewSelfSignedLoader)
}
