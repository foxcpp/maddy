//go:build libdns_alidns || libdns_all
// +build libdns_alidns libdns_all

package libdns

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/framework/module/modules"
	"github.com/libdns/alidns"
)

func init() {
	modules.Register("libdns.alidns", func(c *container.C, modName, instName string) (module.Module, error) {
		p := alidns.Provider{}
		return &ProviderModule{
			RecordDeleter:  &p,
			RecordAppender: &p,
			setConfig: func(c *config.Map) {
				c.String("key_id", false, false, "", &p.AccKeyID)
				c.String("key_secret", false, false, "", &p.AccKeySecret)
			},
			instName: instName,
			modName:  modName,
		}, nil
	})
}
