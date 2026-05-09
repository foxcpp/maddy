package imapsql2

import (
	"context"
	"errors"
	"io"
	"math"
	"strings"

	"github.com/emersion/go-imap/v2/imapserver"
	maddy_storage "github.com/foxcpp/maddy-storage"
	"github.com/foxcpp/maddy-storage/pkg/delivery"
	"github.com/foxcpp/maddy-storage/pkg/imap2"
	"github.com/foxcpp/maddy-storage/pkg/managed"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/framework/module/modules"
	"github.com/foxcpp/maddy/internal/authz"
)

const modName = "storage.imapsql2"

type Storage struct {
	instName string

	store    *maddy_storage.Container
	imap     *imap2.Backend
	imapOpts *imapserver.Options
	delivery *delivery.Container
	manage   *managed.Container

	log *log.Logger

	filters module.IMAPFilter

	deliveryMap       module.Table
	deliveryNormalize func(context.Context, string) (string, error)
	autoCreateAcct    bool
}

func (store *Storage) Name() string         { return modName }
func (store *Storage) InstanceName() string { return store.instName }

func New(c *container.C, _, instName string) (module.Module, error) {
	store := &Storage{
		instName: instName,
		log:      c.DefaultLogger.Sublogger(modName),
	}
	return store, nil
}

func (store *Storage) Configure(inlineArgs []string, cfg *config.Map) error {
	var (
		driver string
		dsn    []string
	)

	if len(inlineArgs) != 0 {
		if len(inlineArgs) == 1 {
			return errors.New("imapsql2: expected at least 2 arguments")
		}

		driver = inlineArgs[0]
		dsn = inlineArgs[1:]
	}

	var (
		blobStore          module.BlobStore
		tempStore          module.BlobStore
		deliveryNormalize  string
		defaultAppendLimit int64
	)

	cfg.Bool("debug", true, false, &store.log.Debug)

	cfg.String("driver", false, false, driver, &driver)
	cfg.StringList("dsn", false, false, dsn, &dsn)

	cfg.Custom("blob_store", false, false, func() (interface{}, error) {
		var store module.BlobStore
		err := modconfig.ModuleFromNode("storage.blob", []string{"fs", "messages"},
			config.Node{}, nil, &store)
		return store, err
	}, func(m *config.Map, node config.Node) (interface{}, error) {
		var store module.BlobStore
		err := modconfig.ModuleFromNode("storage.blob", node.Args,
			node, m.Globals, &store)
		return store, err
	}, &blobStore)
	cfg.Custom("temp_store", false, false, func() (interface{}, error) {
		var store module.BlobStore
		err := modconfig.ModuleFromNode("storage.blob", []string{"fs", "buffer"},
			config.Node{}, nil, &store)
		return store, err
	}, func(m *config.Map, node config.Node) (interface{}, error) {
		var store module.BlobStore
		err := modconfig.ModuleFromNode("storage.blob", node.Args,
			node, m.Globals, &store)
		return store, err
	}, &tempStore)

	cfg.Bool("auto_create_accounts", false, true, &store.autoCreateAcct)
	cfg.DataSize("appendlimit", false, false, 32*1024*1024, &defaultAppendLimit)
	cfg.Custom("imap_filter", false, false, func() (interface{}, error) {
		return nil, nil
	}, func(m *config.Map, node config.Node) (interface{}, error) {
		var filter module.IMAPFilter
		err := modconfig.GroupFromNode("imap_filters", node.Args, node, m.Globals, &filter)
		return filter, err
	}, &store.filters)

	cfg.Custom("delivery_map", false, false, func() (interface{}, error) {
		return nil, nil
	}, modconfig.TableDirective, &store.deliveryMap)
	cfg.String("delivery_normalize", false, false, "precis_casefold_email", &deliveryNormalize)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	deliveryNormFunc, ok := authz.NormalizeFuncs[deliveryNormalize]
	if !ok {
		return errors.New("storage.imapsql2: unknown normalization function: " + deliveryNormalize)
	}
	store.deliveryNormalize = func(ctx context.Context, s string) (string, error) {
		return deliveryNormFunc(s)
	}
	if store.deliveryMap != nil {
		store.deliveryNormalize = func(ctx context.Context, email string) (string, error) {
			email, err := deliveryNormFunc(email)
			if err != nil {
				return "", err
			}
			mapped, ok, err := store.deliveryMap.Lookup(ctx, email)
			if err != nil || !ok {
				return "", userDoesNotExist(err)
			}
			return mapped, nil
		}
	}

	if defaultAppendLimit > math.MaxUint32 {
		return errors.New("storage.imapsql2: appendlimit too big")
	}

	switch driver {
	case "sqlite3", "sqlite":
		st, err := maddy_storage.NewUsingSQLite(
			maddy_storage.SQLiteConfig{},
			strings.Join(dsn, " "),
			maddy_storage.StubAuth{},
			blobStoreAdapter{blobStore: blobStore},
			blobStoreAdapter{blobStore: tempStore},
		)
		if err != nil {
			return err
		}
		store.store = st
	default:
		return errors.New("storage.imapsql2: unknown driver: " + driver)
	}

	store.imap = store.store.IMAP(store.log.Zap(), imap2.Config{
		AutoCreateAccounts: store.autoCreateAcct,
		DefaultAppendLimit: uint32(defaultAppendLimit),
	})
	store.imapOpts = store.imap.Options()
	store.delivery = store.store.Delivery(delivery.Config{}, store.log.Zap())
	store.manage = store.store.Manager()

	return nil
}

type blobStoreAdapter struct {
	blobStore module.BlobStore
}

func (bs blobStoreAdapter) Create(ctx context.Context, path string) (io.WriteCloser, error) {
	b, err := bs.blobStore.Create(ctx, path, module.UnknownBlobSize)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (bs blobStoreAdapter) Open(ctx context.Context, path string) (io.ReadCloser, error) {
	return bs.blobStore.Open(ctx, path)
}

func (bs blobStoreAdapter) Delete(ctx context.Context, path string) error {
	return bs.blobStore.Delete(ctx, []string{path})
}

func init() {
	modules.Register(modName, New)
}
