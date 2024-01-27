//go:build libdns_gandi || !libdns_separate
// +build libdns_gandi !libdns_separate

package libdns

import (
	"fmt"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/log"
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
				c.String("api_token", false, false, "", &p.APIToken)
				c.String("personal_token", false, false, "", &p.BearerToken)
			},
			afterConfig: func() error {
				if p.APIToken != "" {
					log.Println("libdns.gandi: api_token is deprecated, use personal_token instead (https://api.gandi.net/docs/authentication/)")
				}
				if p.APIToken == "" && p.BearerToken == "" {
					return fmt.Errorf("libdns.gandi: either api_token or personal_token should be specified")
				}
				return nil
			},
			instName: instName,
			modName:  modName,
		}, nil
	})
}
