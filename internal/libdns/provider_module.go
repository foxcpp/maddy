package libdns

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/libdns/libdns"
)

type ProviderModule struct {
	libdns.RecordDeleter
	libdns.RecordAppender
	setConfig func(c *config.Map)

	instName string
	modName  string
}

func (p *ProviderModule) Init(cfg *config.Map) error {
	p.setConfig(cfg)
	_, err := cfg.Process()
	return err
}

func (p *ProviderModule) Name() string {
	return p.modName
}

func (p *ProviderModule) InstanceName() string {
	return p.instName
}
