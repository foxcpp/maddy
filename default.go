package maddy

import (
	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/log"
	"github.com/emersion/maddy/module"
)

var defaultDriver, defaultDsn string

func initDefaultStorage(globalCfg map[string]config.Node) {
	if defaultDriver == "" {
		defaultDriver = "sqlite3"
	}
	if defaultDsn == "" {
		defaultDsn = "maddy.db"
	}

	mod, err := NewSQLMail("default", globalCfg, config.Node{ //TODO!
		Name: "sqlmail",
		Args: []string{"default"},
		Children: []config.Node{
			{
				Name: "driver",
				Args: []string{defaultDriver},
			},
			{
				Name: "dsn",
				Args: []string{defaultDsn},
			},
		},
	})

	if err != nil {
		log.Println("failed to initialize default (go-sqlmail) backend:", err)
		return
	}

	module.RegisterInstance(mod)
	module.RegisterInstance(Dummy{instName: "default_remote_delivery"})
}
