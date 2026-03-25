//go:build libdns_leaseweb || libdns_all
// +build libdns_leaseweb libdns_all

package libdns

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/libdns/leaseweb"
)

func init() {
	module.Register("libdns.leaseweb", func(modName, instName string) (module.Module, error) {
		p := leaseweb.Provider{}
		return &ProviderModule{
			RecordDeleter:  &p,
			RecordAppender: &p,
			setConfig: func(c *config.Map) {
				log.DefaultLogger.Println("WARNING: maddy 0.10.0 will drop libdns.leaseweb, see https://github.com/foxcpp/maddy/issues/807 for details")
				c.String("api_key", false, false, "", &p.APIKey)
			},
			instName: instName,
			modName:  modName,
		}, nil
	})
}
