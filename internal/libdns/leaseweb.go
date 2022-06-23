//go:build libdns_leaseweb || libdns_all
// +build libdns_leaseweb libdns_all

package libdns

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/libdns/leaseweb"
)

func init() {
	module.Register("libdns.leaseweb", func(modName, instName string, _, _ []string) (module.Module, error) {
		p := leaseweb.Provider{}
		return &ProviderModule{
			RecordDeleter:  &p,
			RecordAppender: &p,
			setConfig: func(c *config.Map) {
				c.String("api_key", false, false, "", &p.APIKey)
			},
			instName: instName,
			modName:  modName,
		}, nil
	})
}
