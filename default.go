package maddy

import (
	"log"

	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/module"
)

var defaultDriver, defaultDsn string

func init() {
	if defaultDriver == "" {
		defaultDriver = "sqlite3"
	}
	if defaultDsn == "" {
		defaultDsn = "maddy.db"
	}

	mod, err := NewSQLMail("default", config.CfgTreeNode{
		Name: "sqlmail",
		Args: []string{"default"},
		Childrens: []config.CfgTreeNode{
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
	module.RegisterInstance(Dummy{instName: "default-remote-delivery"})
}
