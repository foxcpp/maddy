package libdns

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/libdns/libdns"
)

type ProviderModule struct {
	libdns.RecordDeleter
	libdns.RecordAppender
	setConfig   func(c *config.Map)
	afterConfig func() error

	instName string
	modName  string
}

func (p *ProviderModule) Configure(inlineArgs []string, cfg *config.Map) error {
	p.setConfig(cfg)
	_, err := cfg.Process()
	if p.afterConfig != nil {
		if err := p.afterConfig(); err != nil {
			return err
		}
	}
	return err
}

func (p *ProviderModule) Name() string {
	return p.modName
}

func (p *ProviderModule) InstanceName() string {
	return p.instName
}
