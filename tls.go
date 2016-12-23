package maddy

import (
	"crypto/tls"
	"fmt"

	"github.com/mholt/caddy/caddyfile"
)

func getTLSConfig(d caddyfile.Dispenser) (*tls.Config, error) {
	config := new(tls.Config)

	for d.Next() {
		args := d.RemainingArgs()
		switch len(args) {
		case 1:
			switch args[0] {
			case "off":
				return nil, nil
			case "self_signed":
				if err := makeSelfSignedCert(config); err != nil {
					return nil, err
				}
			}
		case 2:
			if cert, err := tls.LoadX509KeyPair(args[0], args[1]); err != nil {
				return nil, err
			} else {
				config.Certificates = append(config.Certificates, cert)
			}
		}
	}

	return config, nil
}

func makeSelfSignedCert(config *tls.Config) error {
	return fmt.Errorf("Not yet implemented")
}
