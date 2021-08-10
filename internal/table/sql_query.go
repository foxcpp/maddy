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
	"database/sql"
	"fmt"
	"strings"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
	_ "github.com/lib/pq"
)

type SQL struct {
	modName  string
	instName string

	namedArgs bool

	db     *sql.DB
	lookup *sql.Stmt
	add    *sql.Stmt
	list   *sql.Stmt
	set    *sql.Stmt
	del    *sql.Stmt
}

func NewSQL(modName, instName string, _, _ []string) (module.Module, error) {
	return &SQL{
		modName:  modName,
		instName: instName,
	}, nil
}

func (s *SQL) Name() string {
	return s.modName
}

func (s *SQL) InstanceName() string {
	return s.instName
}

func (s *SQL) Init(cfg *config.Map) error {
	var (
		driver      string
		initQueries []string
		dsnParts    []string
		lookupQuery string

		addQuery    string
		listQuery   string
		removeQuery string
		setQuery    string
	)
	cfg.StringList("init", false, false, nil, &initQueries)
	cfg.String("driver", false, true, "", &driver)
	cfg.StringList("dsn", false, true, nil, &dsnParts)
	cfg.Bool("named_args", false, false, &s.namedArgs)

	cfg.String("lookup", false, true, "", &lookupQuery)

	cfg.String("add", false, false, "", &addQuery)
	cfg.String("list", false, false, "", &listQuery)
	cfg.String("del", false, false, "", &removeQuery)
	cfg.String("set", false, false, "", &setQuery)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	if driver == "postgres" && s.namedArgs {
		return config.NodeErr(cfg.Block, "PostgreSQL driver does not support named_args")
	}

	db, err := sql.Open(driver, strings.Join(dsnParts, " "))
	if err != nil {
		return config.NodeErr(cfg.Block, "failed to open db: %v", err)
	}
	s.db = db

	for _, init := range initQueries {
		if _, err := db.Exec(init); err != nil {
			return config.NodeErr(cfg.Block, "init query failed: %v", err)
		}
	}

	s.lookup, err = db.Prepare(lookupQuery)
	if err != nil {
		return config.NodeErr(cfg.Block, "failed to prepare lookup query: %v", err)
	}
	if addQuery != "" {
		s.add, err = db.Prepare(addQuery)
		if err != nil {
			return config.NodeErr(cfg.Block, "failed to prepare add query: %v", err)
		}
	}
	if listQuery != "" {
		s.list, err = db.Prepare(listQuery)
		if err != nil {
			return config.NodeErr(cfg.Block, "failed to prepare list query: %v", err)
		}
	}
	if setQuery != "" {
		s.set, err = db.Prepare(setQuery)
		if err != nil {
			return config.NodeErr(cfg.Block, "failed to prepare set query: %v", err)
		}
	}
	if removeQuery != "" {
		s.del, err = db.Prepare(removeQuery)
		if err != nil {
			return config.NodeErr(cfg.Block, "failed to prepare del query: %v", err)
		}
	}

	return nil
}

func (s *SQL) Close() error {
	s.lookup.Close()
	return s.db.Close()
}

func (s *SQL) Lookup(ctx context.Context, val string) (string, bool, error) {
	var (
		repl string
		row  *sql.Row
	)
	if s.namedArgs {
		row = s.lookup.QueryRowContext(ctx, sql.Named("key", val))
	} else {
		row = s.lookup.QueryRowContext(ctx, val)
	}
	if err := row.Scan(&repl); err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, fmt.Errorf("%s: lookup %s: %w", s.modName, val, err)
	}
	return repl, true, nil
}

func (s *SQL) LookupMulti(ctx context.Context, val string) ([]string, error) {
	var (
		repl []string
		rows *sql.Rows
		err  error
	)
	if s.namedArgs {
		rows, err = s.lookup.QueryContext(ctx, sql.Named("key", val))
	} else {
		rows, err = s.lookup.QueryContext(ctx, val)
	}
	if err != nil {
		return nil, fmt.Errorf("%s; lookup %s: %w", s.modName, val, err)
	}
	for rows.Next() {
		var res string
		if err := rows.Scan(&res); err != nil {
			return nil, fmt.Errorf("%s; lookup %s: %w", s.modName, val, err)
		}
		repl = append(repl, res)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s; lookup %s: %w", s.modName, val, err)
	}
	return repl, nil
}

func (s *SQL) Keys() ([]string, error) {
	if s.list == nil {
		return nil, fmt.Errorf("%s: table is not mutable (no 'list' query)", s.modName)
	}

	rows, err := s.list.Query()
	if err != nil {
		return nil, fmt.Errorf("%s: list: %w", s.modName, err)
	}
	defer rows.Close()
	var list []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("%s: list: %w", s.modName, err)
		}
		list = append(list, key)
	}
	return list, nil
}

func (s *SQL) RemoveKey(k string) error {
	if s.del == nil {
		return fmt.Errorf("%s: table is not mutable (no 'del' query)", s.modName)
	}

	var err error
	if s.namedArgs {
		_, err = s.del.Exec(sql.Named("key", k))
	} else {
		_, err = s.del.Exec(k)
	}
	if err != nil {
		return fmt.Errorf("%s: del %s: %w", s.modName, k, err)
	}
	return nil
}

func (s *SQL) SetKey(k, v string) error {
	if s.set == nil {
		return fmt.Errorf("%s: table is not mutable (no 'set' query)", s.modName)
	}
	if s.add == nil {
		return fmt.Errorf("%s: table is not mutable (no 'add' query)", s.modName)
	}

	var args []interface{}
	if s.namedArgs {
		args = []interface{}{sql.Named("key", k), sql.Named("value", v)}
	} else {
		args = []interface{}{k, v}
	}

	if _, err := s.add.Exec(args...); err != nil {
		if _, err := s.set.Exec(args...); err != nil {
			return fmt.Errorf("%s: add %s: %w", s.modName, k, err)
		}
		return nil
	}
	return nil
}

func init() {
	module.Register("table.sql_query", NewSQL)
}
