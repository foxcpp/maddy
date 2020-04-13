package table

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/module"
	_ "github.com/lib/pq"
)

type SQL struct {
	modName  string
	instName string

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

	cfg.String("lookup", false, true, "", &lookupQuery)

	cfg.String("add", false, false, "", &addQuery)
	cfg.String("list", false, false, "", &listQuery)
	cfg.String("del", false, false, "", &removeQuery)
	cfg.String("set", false, false, "", &setQuery)
	if _, err := cfg.Process(); err != nil {
		return err
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

func (s *SQL) Lookup(val string) (string, bool, error) {
	var repl string
	row := s.lookup.QueryRow(val)
	if err := row.Scan(&repl); err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, fmt.Errorf("%s: lookup %s: %w", s.modName, val, err)
	}
	return repl, true, nil
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

	_, err := s.del.Exec(k)
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

	if _, err := s.add.Exec(k, v); err != nil {
		if _, err := s.set.Exec(k, v); err != nil {
			return fmt.Errorf("%s: add %s: %w", s.modName, k, err)
		}
		return nil
	}
	return nil
}

func init() {
	module.Register("sql_query", NewSQL)
}
