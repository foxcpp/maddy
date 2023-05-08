package libdns

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/libdns/inwx"
)

func init() {
	module.Register("libdns.inwx", func(modName, instName string, _, _ []string) (module.Module, error) {
		p := inwx.Provider{}
		return &ProviderModule{
			RecordDeleter:  &p,
			RecordAppender: &p,
			setConfig: func(c *config.Map) {
				c.String("username", false, true, "", &p.Username)
				c.String("password", false, true, "", &p.Password)
				c.String("shared_secret", false, false, "", &p.SharedSecret)
			},
			instName: instName,
			modName:  modName,
		}, nil
	})
}
