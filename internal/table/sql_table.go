package table

import (
	"fmt"

	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/module"
	_ "github.com/lib/pq"
)

type SQLTable struct {
	modName  string
	instName string

	wrapped *SQL
}

func NewSQLTable(modName, instName string, _, _ []string) (module.Module, error) {
	return &SQLTable{
		modName:  modName,
		instName: instName,

		wrapped: &SQL{
			modName:  modName,
			instName: instName,
		},
	}, nil
}

func (s *SQLTable) Name() string {
	return s.modName
}

func (s *SQLTable) InstanceName() string {
	return s.instName
}

func (s *SQLTable) Init(cfg *config.Map) error {
	var (
		driver      string
		dsnParts    []string
		tableName   string
		keyColumn   string
		valueColumn string
	)
	cfg.String("driver", false, true, "", &driver)
	cfg.StringList("dsn", false, true, nil, &dsnParts)
	cfg.String("table_name", false, true, "", &tableName)
	cfg.String("key_column", false, false, "key", &keyColumn)
	cfg.String("value_column", false, false, "value", &valueColumn)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	// sql_table module literally wraps the sql_query module by generating a
	// configuration block for it.

	return s.wrapped.Init(config.NewMap(cfg.Globals, config.Node{
		Children: []config.Node{
			{
				Name: "driver",
				Args: []string{driver},
			},
			{
				Name: "dsn",
				Args: dsnParts,
			},
			{
				Name: "lookup",
				Args: []string{fmt.Sprintf("SELECT %s FROM %s WHERE %s = :key", valueColumn, tableName, keyColumn)},
			},
			{
				Name: "add",
				Args: []string{fmt.Sprintf("INSERT INTO %s(%s, %s) VALUES(:key, :value)", tableName, keyColumn, valueColumn)},
			},
			{
				Name: "list",
				Args: []string{fmt.Sprintf("SELECT %s from %s", keyColumn, tableName)},
			},
			{
				Name: "set",
				Args: []string{fmt.Sprintf("UPDATE %s SET %s = :value WHERE %s = :key", tableName, valueColumn, keyColumn)},
			},
			{
				Name: "del",
				Args: []string{fmt.Sprintf("DELETE FROM %s WHERE %s = :key", tableName, keyColumn)},
			},
			{
				Name: "init",
				Args: []string{fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
					%s LONGTEXT PRIMARY KEY NOT NULL,
					%s LONGTEXT NOT NULL
				)`, tableName, keyColumn, valueColumn)},
			},
		},
	}))
}

func (s *SQLTable) Close() error {
	return s.wrapped.Close()
}

func (s *SQLTable) Lookup(val string) (string, bool, error) {
	return s.wrapped.Lookup(val)
}

func (s *SQLTable) Keys() ([]string, error) {
	return s.wrapped.Keys()
}

func (s *SQLTable) RemoveKey(k string) error {
	return s.wrapped.RemoveKey(k)
}

func (s *SQLTable) SetKey(k, v string) error {
	return s.wrapped.SetKey(k, v)
}

func init() {
	module.RegisterDeprecated("sql_table", "table.sql_table", NewSQLTable)
	module.Register("table.sql_table", NewSQLTable)
}
