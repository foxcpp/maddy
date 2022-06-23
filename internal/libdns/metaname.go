//go:build libdns_metaname || libdns_all
// +build libdns_metaname libdns_all

package libdns

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/libdns/metaname"
)

func init() {
	module.Register("libdns.metaname", func(modName, instName string, _, _ []string) (module.Module, error) {
		p := metaname.Provider{
			Endpoint: "https://metaname.net/api/1.1",
		}
		return &ProviderModule{
			RecordDeleter:  &p,
			RecordAppender: &p,
			setConfig: func(c *config.Map) {
				c.String("api_key", false, false, "", &p.APIKey)
				c.String("account_ref", false, false, "", &p.AccountReference)
			},
			instName: instName,
			modName:  modName,
		}, nil
	})
}
