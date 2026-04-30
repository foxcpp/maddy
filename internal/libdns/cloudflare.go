//go:build libdns_cloudflare || !libdns_separate
// +build libdns_cloudflare !libdns_separate

package libdns

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/framework/module/modules"
	"github.com/libdns/cloudflare"
)

func init() {
	modules.Register("libdns.cloudflare", func(c *container.C, modName, instName string) (module.Module, error) {
		p := cloudflare.Provider{}
		return &ProviderModule{
			RecordDeleter:  &p,
			RecordAppender: &p,
			setConfig: func(c *config.Map) {
				c.String("api_token", false, false, "", &p.APIToken)
			},
			instName: instName,
			modName:  modName,
		}, nil
	})
}
