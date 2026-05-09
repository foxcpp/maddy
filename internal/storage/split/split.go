package split

import (
	"context"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/framework/module/modules"
	"github.com/foxcpp/maddy/framework/module/storage"
	"github.com/foxcpp/maddy/internal/updatepipe"
)

const modName = "storage.split"

type split struct {
	instName string
	mapping  module.Table
	named    map[string]storage.Storage
	unnamed  storage.Storage

	logger *log.Logger
}

func (s *split) Configure(_ []string, cfg *config.Map) error {
	modconfig.Module[module.Table](cfg, "mapping", false, false, "table", nil, &s.mapping)
	cfg.Callback("named", func(_ *config.Map, node config.Node) error {
		if len(node.Args) < 2 {
			return config.NodeErr(node, "expected at least one value")
		}

		if s.named == nil {
			s.named = make(map[string]storage.Storage)
		}

		name := node.Args[0]
		node.Args = node.Args[1:]

		store, err := modconfig.ModuleDirective[storage.Storage]("storage", cfg, node)
		if err != nil {
			return err
		}

		s.named[name] = store
		return nil
	})
	modconfig.Module[storage.Storage](cfg, "default", false, false, "storage", nil, &s.unnamed)

	if _, err := cfg.Process(); err != nil {
		return err
	}
	return nil
}

func (s *split) Name() string {
	return modName
}

func (s *split) InstanceName() string {
	return s.instName
}

func New(c *container.C, modName, instName string) (module.Module, error) {
	return &split{
		instName: instName,
		logger:   c.DefaultLogger.Sublogger(modName),
	}, nil
}

func (s *split) EnableUpdatePipe(mode updatepipe.BackendMode) error {
	for _, store := range s.named {
		if upd, ok := store.(updatepipe.Backend); ok {
			if err := upd.EnableUpdatePipe(mode); err != nil {
				return err
			}
		}
	}

	if upd, ok := s.unnamed.(updatepipe.Backend); ok {
		if err := upd.EnableUpdatePipe(mode); err != nil {
			return err
		}
	}

	return nil
}

func (s *split) NewSession(ctx context.Context, conn *imapserver.Conn) (storage.Session, *imapserver.GreetingData, error) {
	return &deferredSession{
		s:       s,
		conn:    conn,
		Session: nil,
	}, &imapserver.GreetingData{}, nil
}

func (s *split) storeForAccount(ctx context.Context, accountID string) (storage.Storage, error) {
	if s.mapping != nil {
		value, ok, err := s.mapping.Lookup(ctx, accountID)
		if err != nil {
			return nil, err
		}
		if ok {
			store, ok := s.named[value]
			if !ok {
				s.logger.Msg("missing named storage %s", value)
				return nil, imapserver.ErrAuthFailed
			}
			return store, nil
		}
	}

	if s.unnamed != nil {
		return s.unnamed, nil
	}

	return nil, imapserver.ErrAuthFailed
}

func (s *split) AvailableCaps() imap.CapSet {
	// Intersection of all backend caps.
	// Initialized as a list of extensions proxied
	// by deferredSession to avoid interface mismatches.
	inter := imap.CapSet{
		imap.CapIMAP4rev2:      {},
		imap.CapAppendLimit:    {},
		imap.CapMove:           {},
		imap.CapUnauthenticate: {},
		imap.CapBinary:         {},
		imap.CapUIDPlus:        {},
		imap.CapCondStore:      {},
		imap.CapESearch:        {},
		imap.CapESort:          {},
		imap.CapNamespace:      {},
	}

	for _, store := range s.named {
		caps := store.AvailableCaps()
		if inter == nil {
			inter = caps
			continue
		}

		for c := range inter {
			if !caps.Has(c) {
				delete(inter, c)
			}
		}
	}
	if s.unnamed != nil {
		caps := s.unnamed.AvailableCaps()
		if inter == nil {
			inter = caps
		} else {
			for c := range inter {
				if !caps.Has(c) {
					delete(inter, c)
				}
			}
		}
	}

	return inter
}

func init() {
	modules.Register(modName, New)
}
