//go:build libdns_route53 || libdns_all
// +build libdns_route53 libdns_all

package libdns

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/framework/module/modules"
	"github.com/libdns/route53"
)

func init() {
	modules.Register("libdns.route53", func(c *container.C, modName, instName string) (module.Module, error) {
		p := route53.Provider{}
		return &ProviderModule{
			RecordDeleter:  &p,
			RecordAppender: &p,
			setConfig: func(c *config.Map) {
				c.String("secret_access_key", false, false, "", &p.SecretAccessKey)
				c.String("access_key_id", false, false, "", &p.AccessKeyId)
			},
			instName: instName,
			modName:  modName,
		}, nil
	})
}
