//go:build libdns_namedotdom || libdns_all
// +build libdns_namedotdom libdns_all

package libdns

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/libdns/namedotcom"
)

func init() {
	module.Register("libdns.namedotcom", func(modName, instName string) (module.Module, error) {
		p := namedotcom.Provider{
			Server: "https://api.name.com",
		}
		return &ProviderModule{
			RecordDeleter:  &p,
			RecordAppender: &p,
			setConfig: func(c *config.Map) {
				log.DefaultLogger.Println("WARNING: maddy 0.10.0 will drop libdns.namedotcom, see https://github.com/foxcpp/maddy/issues/807 for details")
				c.String("user", false, false, "", &p.User)
				c.String("token", false, false, "", &p.Token)
			},
			instName: instName,
			modName:  modName,
		}, nil
	})
}
