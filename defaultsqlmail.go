package maddy

import (
	"log"

	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/module"
	_ "github.com/mattn/go-sqlite3"
)

// TODO: We need a good way to set these on compile-time.

const defaultDriver = "sqlite3"
const defaultDsn = "maddy.db"

func init() {
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
		log.Println("failed to initialize default (go-sqlmail) backend: %v", err)
		return
	}

	module.RegisterInstance(mod)
}
