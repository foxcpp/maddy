package maddy

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/module"
)

var defaultDriver = "sqlite3"
var defaultDsn string

func createDefaultStorage(globals *config.Map, _ string) (module.Module, error) {
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

func defaultStorageConfig(globals *config.Map, name string) config.Node {
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
				Args: []string{filepath.Join(StateDirectory(globals.Values), "maddy.db")},
			},
		},
	}
}

func createDefaultRemoteDelivery(_ *config.Map, name string) (module.Module, error) {
	return Dummy{instName: name}, nil
}
