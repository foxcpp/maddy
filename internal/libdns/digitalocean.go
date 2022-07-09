//go:build libdns_digitalocean || !libdns_separate
// +build libdns_digitalocean !libdns_separate

package libdns

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/libdns/digitalocean"
)

func init() {
	module.Register("libdns.digitalocean", func(modName, instName string, _, _ []string) (module.Module, error) {
		p := digitalocean.Provider{}
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
