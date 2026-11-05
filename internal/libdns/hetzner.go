//go:build libdns_hetzner || !libdns_separate
// +build libdns_hetzner !libdns_separate

package libdns

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/framework/module/modules"
	"github.com/libdns/hetzner"
)

func init() {
	modules.Register("libdns.hetzner", func(c *container.C, modName, instName string) (module.Module, error) {
		p := hetzner.Provider{}
		return &ProviderModule{
			RecordDeleter:  &p,
			RecordAppender: &p,
			setConfig: func(c *config.Map) {
				c.String("api_token", false, false, "", &p.AuthAPIToken)
			},
			instName: instName,
			modName:  modName,
		}, nil
	})
}
