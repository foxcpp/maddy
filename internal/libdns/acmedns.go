//go:build libdns_acmedns || libdns_all
// +build libdns_acmedns libdns_all

package libdns

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/framework/module/modules"
	"github.com/libdns/acmedns"
)

func init() {
	modules.Register("libdns.acmedns", func(c *container.C, modName, instName string) (module.Module, error) {
		p := acmedns.Provider{}
		return &ProviderModule{
			RecordDeleter:  &p,
			RecordAppender: &p,
			setConfig: func(c *config.Map) {
				c.String("username", false, true, "", &p.Username)
				c.String("password", false, true, "", &p.Password)
				c.String("subdomain", false, true, "", &p.Subdomain)
				c.String("server_url", false, true, "", &p.ServerURL)
			},
			instName: instName,
			modName:  modName,
		}, nil
	})
}
