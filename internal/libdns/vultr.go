//go:build libdns_vultr || !libdns_separate
// +build libdns_vultr !libdns_separate

package libdns

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/libdns/vultr"
)

func init() {
	module.Register("libdns.vultr", func(modName, instName string) (module.Module, error) {
		p := vultr.Provider{}
		return &ProviderModule{
			RecordDeleter:  &p,
			RecordAppender: &p,
			setConfig: func(c *config.Map) {
				log.DefaultLogger.Println("WARNING: maddy 0.10.0 will drop libdns.vultr, see https://github.com/foxcpp/maddy/issues/807 for details")
				c.String("api_token", false, false, "", &p.APIToken)
			},
			instName: instName,
			modName:  modName,
		}, nil
	})
}
