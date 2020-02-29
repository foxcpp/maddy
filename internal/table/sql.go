package table

import (
	"database/sql"
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
	)
	cfg.StringList("init", false, false, nil, &initQueries)
	cfg.String("driver", false, true, "", &driver)
	cfg.StringList("dsn", false, true, nil, &dsnParts)
	cfg.String("lookup", false, true, "", &lookupQuery)
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
		return "", false, err
	}
	return repl, true, nil
}

func init() {
	module.Register("sql_table", NewSQL)
}
