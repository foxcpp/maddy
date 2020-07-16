package tls

import (
	"crypto/tls"
	"os"
	"strings"

	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
)

type TLSConfig struct {
	loader  module.TLSLoader
	baseCfg *tls.Config
}

func (cfg *TLSConfig) Get() (*tls.Config, error) {
	if cfg.loader == nil {
		return nil, nil
	}
	tlsCfg := cfg.baseCfg.Clone()

	certs, err := cfg.loader.LoadCerts()
	if err != nil {
		return nil, err
	}
	tlsCfg.Certificates = certs

	return tlsCfg, nil
}

// TLSDirective reads the TLS configuration and adds the reload handler to
// reread certificates on SIGUSR2.
//
// The returned value is *tls.TLSConfig with GetConfigForClient set.
// If the 'tls off' is used, returned value is nil.
func TLSDirective(m *config.Map, node config.Node) (interface{}, error) {
	cfg, err := readTLSBlock(m.Globals, node)
	if err != nil {
		return nil, err
	}

	if cfg == nil {
		return nil, nil
	}

	return &tls.Config{
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			return cfg.Get()
		},
	}, nil
}

func readTLSBlock(globals map[string]interface{}, blockNode config.Node) (*TLSConfig, error) {
	baseCfg := tls.Config{}

	var loader module.TLSLoader
	if len(blockNode.Args) > 0 {
		if blockNode.Args[0] == "off" {
			return nil, nil
		}

		if _, err := os.Stat(blockNode.Args[0]); err == nil || strings.Contains(blockNode.Args[0], "/") {
			log.Println("'tls cert_path key_path' syntax is deprecated, use 'tls file cert_path key_path'")
			blockNode.Args = append([]string{"file"}, blockNode.Args...)
		}

		err := modconfig.ModuleFromNode("tls.loader", blockNode.Args, config.Node{}, globals, &loader)
		if err != nil {
			return nil, err
		}
	}

	childM := config.NewMap(globals, blockNode)
	var tlsVersions [2]uint16

	childM.Custom("loader", false, false, func() (interface{}, error) {
		return loader, nil
	}, func(m *config.Map, node config.Node) (interface{}, error) {
		var l module.TLSLoader
		err := modconfig.ModuleFromNode("tls.loader", blockNode.Args, config.Node{}, globals, &l)
		return l, err
	}, &loader)

	childM.Custom("protocols", false, false, func() (interface{}, error) {
		return [2]uint16{0, 0}, nil
	}, TLSVersionsDirective, &tlsVersions)

	childM.Custom("ciphers", false, false, func() (interface{}, error) {
		return nil, nil
	}, TLSCiphersDirective, &baseCfg.CipherSuites)

	childM.Custom("curves", false, false, func() (interface{}, error) {
		return nil, nil
	}, TLSCurvesDirective, &baseCfg.CurvePreferences)

	if _, err := childM.Process(); err != nil {
		return nil, err
	}

	if len(baseCfg.CipherSuites) != 0 {
		baseCfg.PreferServerCipherSuites = true
	}

	baseCfg.MinVersion = tlsVersions[0]
	baseCfg.MaxVersion = tlsVersions[1]
	log.Debugf("tls: min version: %x, max version: %x", tlsVersions[0], tlsVersions[1])

	return &TLSConfig{
		loader:  loader,
		baseCfg: &baseCfg,
	}, nil
}
