/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package table

import (
	"context"
	"fmt"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
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

	var (
		useNamedArgs string

		lookupQuery string
		addQuery    string
		listQuery   string
		setQuery    string
		delQuery    string
	)
	if driver == "sqlite3" {
		useNamedArgs = "yes"
		lookupQuery = fmt.Sprintf("SELECT %s FROM %s WHERE %s = :key", valueColumn, tableName, keyColumn)
		addQuery = fmt.Sprintf("INSERT INTO %s(%s, %s) VALUES(:key, :value)", tableName, keyColumn, valueColumn)
		listQuery = fmt.Sprintf("SELECT %s from %s", keyColumn, tableName)
		setQuery = fmt.Sprintf("UPDATE %s SET %s = :value WHERE %s = :key", tableName, valueColumn, keyColumn)
		delQuery = fmt.Sprintf("DELETE FROM %s WHERE %s = :key", tableName, keyColumn)
	} else {
		useNamedArgs = "no"
		lookupQuery = fmt.Sprintf("SELECT %s FROM %s WHERE %s = $1", valueColumn, tableName, keyColumn)
		addQuery = fmt.Sprintf("INSERT INTO %s(%s, %s) VALUES($1, $2)", tableName, keyColumn, valueColumn)
		listQuery = fmt.Sprintf("SELECT %s from %s", keyColumn, tableName)
		setQuery = fmt.Sprintf("UPDATE %s SET %s = $2 WHERE %s = $1", tableName, valueColumn, keyColumn)
		delQuery = fmt.Sprintf("DELETE FROM %s WHERE %s = $1", tableName, keyColumn)
	}

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
				Name: "named_args",
				Args: []string{useNamedArgs},
			},
			{
				Name: "lookup",
				Args: []string{lookupQuery},
			},
			{
				Name: "add",
				Args: []string{addQuery},
			},
			{
				Name: "list",
				Args: []string{listQuery},
			},
			{
				Name: "set",
				Args: []string{setQuery},
			},
			{
				Name: "del",
				Args: []string{delQuery},
			},
			{
				Name: "init",
				Args: []string{fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
					%s TEXT PRIMARY KEY NOT NULL,
					%s TEXT NOT NULL
				)`, tableName, keyColumn, valueColumn)},
			},
		},
	}))
}

func (s *SQLTable) Close() error {
	return s.wrapped.Close()
}

func (s *SQLTable) Lookup(ctx context.Context, val string) (string, bool, error) {
	return s.wrapped.Lookup(ctx, val)
}

func (s *SQLTable) LookupMulti(ctx context.Context, val string) ([]string, error) {
	return s.wrapped.LookupMulti(ctx, val)
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
	module.Register("table.sql_table", NewSQLTable)
}
