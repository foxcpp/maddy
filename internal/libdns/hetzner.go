//go:build libdns_hetzner || !libdns_separate
// +build libdns_hetzner !libdns_separate

package libdns

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/libdns/hetzner"
)

func init() {
	module.Register("libdns.hetzner", func(modName, instName string, _, _ []string) (module.Module, error) {
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
