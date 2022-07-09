//go:build libdns_googleclouddns || libdns_all
// +build libdns_googleclouddns libdns_all

package libdns

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/libdns/googleclouddns"
)

func init() {
	module.Register("libdns.googleclouddns", func(modName, instName string, _, _ []string) (module.Module, error) {
		p := googleclouddns.Provider{}
		return &ProviderModule{
			RecordDeleter:  &p,
			RecordAppender: &p,
			setConfig: func(c *config.Map) {
				c.String("project", false, true, "", &p.Project)
				c.String("service_account_json", false, false, "", &p.ServiceAccountJSON)
			},
			instName: instName,
			modName:  modName,
		}, nil
	})
}
