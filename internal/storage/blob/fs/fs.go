package fs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/framework/module/modules"
)

const modName = "storage.blob.fs"

// FSStore struct represents directory on FS used to store blobs.
type FSStore struct {
	log      *log.Logger
	instName string
	rootPath string
	root     *os.Root
}

func New(c *container.C, _, instName string) (module.Module, error) {
	return &FSStore{
		log:      c.DefaultLogger.Sublogger(modName),
		instName: instName,
	}, nil
}

func (s *FSStore) Name() string {
	return "storage.blob.fs"
}

func (s *FSStore) InstanceName() string {
	return s.instName
}

func (s *FSStore) Configure(inlineArgs []string, cfg *config.Map) error {
	switch len(inlineArgs) {
	case 0:
	case 1:
		s.rootPath = inlineArgs[0]
	default:
		return fmt.Errorf("storage.blob.fs: 1 or 0 arguments expected")
	}

	cfg.String("root", false, false, s.rootPath, &s.rootPath)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	if s.rootPath == "" {
		return config.NodeErr(cfg.Block, "storage.blob.fs: directory not set")
	}

	s.rootPath = filepath.Clean(s.rootPath)
	if err := os.MkdirAll(s.rootPath, os.ModeDir|os.ModePerm); err != nil {
		return err
	}

	var err error
	s.root, err = os.OpenRoot(s.rootPath)
	if err != nil {
		return err
	}

	return nil
}

func (s *FSStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	f, err := s.root.Open(key)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, module.ErrNoSuchBlob
		}
		return nil, err
	}
	return f, nil
}

func (s *FSStore) Create(_ context.Context, key string, blobSize int64) (module.Blob, error) {
	f, err := s.root.Create(key)
	if err != nil {
		return nil, err
	}
	if blobSize >= 0 {
		if err := f.Truncate(blobSize); err != nil {
			_ = f.Close()
			return nil, err
		}
	}
	return f, nil
}

func (s *FSStore) Delete(_ context.Context, keys []string) error {
	for _, key := range keys {
		if err := s.root.Remove(key); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
	}
	return nil
}

func (s *FSStore) Lookup(_ context.Context, key string) (string, bool, error) {
	blob, err := s.root.ReadFile(key)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return string(blob), true, nil
}

func init() {
	var _ module.BlobStore = &FSStore{}
	modules.Register(modName, New)
	modules.Register("table.fs", New)
}
