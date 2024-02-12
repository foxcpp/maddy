//go:build libdns_rfc2136 || libdns_all
// +build libdns_rfc2136 libdns_all

package libdns

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/libdns/rfc2136"
)

func init() {
	module.Register("libdns.rfc2136", func(modName, instName string, _, _ []string) (module.Module, error) {
		p := rfc2136.Provider{}
		return &ProviderModule{
			RecordDeleter:  &p,
			RecordAppender: &p,
			setConfig: func(c *config.Map) {
				c.String("KeyName", false, true, "", &p.KeyName)
				c.String("Key", false, true, "", &p.Key)
				c.String("KeyAlg", false, true, "", &p.KeyAlg)
				c.String("Server", false, true, "", &p.Server)
			},
			instName: instName,
			modName:  modName,
		}, nil
	})
}
