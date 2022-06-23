//go:build go1.16
// +build go1.16

package libdns

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/libdns/namecheap"
)

func init() {
	module.Register("libdns.namecheap", func(modName, instName string, _, _ []string) (module.Module, error) {
		p := namecheap.Provider{}
		return &ProviderModule{
			RecordDeleter:  &p,
			RecordAppender: &p,
			setConfig: func(c *config.Map) {
				c.String("api_key", false, true, "", &p.APIKey)
				c.String("api_username", false, true, "", &p.User)
				c.String("endpoint", false, false, "", &p.APIEndpoint)
				c.String("client_ip", false, false, "", &p.ClientIP)
			},
			instName: instName,
			modName:  modName,
		}, nil
	})
}
