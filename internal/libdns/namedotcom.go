//go:build libdns_namedotdom || libdns_all
// +build libdns_namedotdom libdns_all

package libdns

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/framework/module/modules"
	"github.com/libdns/namedotcom"
)

func init() {
	modules.Register("libdns.namedotcom", func(c *container.C, modName, instName string) (module.Module, error) {
		p := namedotcom.Provider{
			Server: "https://api.name.com",
		}
		return &ProviderModule{
			RecordDeleter:  &p,
			RecordAppender: &p,
			setConfig: func(c *config.Map) {
				c.String("user", false, false, "", &p.User)
				c.String("token", false, false, "", &p.Token)
			},
			instName: instName,
			modName:  modName,
		}, nil
	})
}
