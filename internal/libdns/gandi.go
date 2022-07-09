//go:build libdns_gandi || !libdns_separate
// +build libdns_gandi !libdns_separate

package libdns

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/libdns/gandi"
)

func init() {
	module.Register("libdns.gandi", func(modName, instName string, _, _ []string) (module.Module, error) {
		p := gandi.Provider{}
		return &ProviderModule{
			RecordDeleter:  &p,
			RecordAppender: &p,
			setConfig: func(c *config.Map) {
				c.String("api_token", false, true, "", &p.APIToken)
			},
			instName: instName,
			modName:  modName,
		}, nil
	})
}
