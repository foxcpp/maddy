//go:build libdns_gcore || !libdns_separate

package libdns

import (
	"fmt"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/framework/module/modules"
	"github.com/libdns/gcore"
)

func init() {
	modules.Register("libdns.gcore", func(c *container.C, modName, instName string) (module.Module, error) {
		p := gcore.Provider{}
		return &ProviderModule{
			RecordDeleter:  &p,
			RecordAppender: &p,
			setConfig: func(c *config.Map) {
				c.String("api_key", false, false, "", &p.APIKey)
			},
			afterConfig: func() error {
				if p.APIKey == "" {
					return fmt.Errorf("libdns.gcore: api_key should be specified")
				}
				return nil
			},

			instName: instName,
			modName:  modName,
		}, nil
	})
}
