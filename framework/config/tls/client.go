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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/log"
)

func TLSClientBlock(_ *config.Map, node config.Node) (interface{}, error) {
	cfg := tls.Config{}

	childM := config.NewMap(nil, node)
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

	if _, err := childM.Process(); err != nil {
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

	if certPath != "" && keyPath != "" {
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
