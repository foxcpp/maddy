package maddy

import (
	"database/sql"
	"fmt"

	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/module"
)

var defaultDriver, defaultDsn string

func createDefaultStorage(_ string) (module.Module, error) {
	if defaultDriver == "" {
		defaultDriver = "sqlite3"
	}
	if defaultDsn == "" {
		defaultDsn = "maddy.db"
	}

	driverSupported := false
	for _, driver := range sql.Drivers() {
		if driver == defaultDriver {
			driverSupported = true
		}
	}

	if !driverSupported {
		return nil, fmt.Errorf("maddy is not compiled with %s support", defaultDriver)
	}

	return NewSQLStorage("sql", "default")
}

func defaultStorageConfig(name string) config.Node {
	return config.Node{
		Name: "sql",
		Args: []string{name},
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
	}
}

func createDefaultRemoteDelivery(name string) (module.Module, error) {
	return Dummy{instName: name}, nil
}
