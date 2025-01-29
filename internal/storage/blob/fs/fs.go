package fs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
)

// FSStore struct represents directory on FS used to store blobs.
type FSStore struct {
	instName string
	root     string
}

func New(_, instName string) (module.Module, error) {
	return &FSStore{instName: instName}, nil
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
		s.root = inlineArgs[0]
	default:
		return fmt.Errorf("storage.blob.fs: 1 or 0 arguments expected")
	}

	cfg.String("root", false, false, s.root, &s.root)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	if s.root == "" {
		return config.NodeErr(cfg.Block, "storage.blob.fs: directory not set")
	}

	if err := os.MkdirAll(s.root, os.ModeDir|os.ModePerm); err != nil {
		return err
	}

	return nil
}

func (s *FSStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	f, err := os.Open(filepath.Join(s.root, key))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, module.ErrNoSuchBlob
		}
		return nil, err
	}
	return f, nil
}

func (s *FSStore) Create(_ context.Context, key string, blobSize int64) (module.Blob, error) {
	f, err := os.Create(filepath.Join(s.root, key))
	if err != nil {
		return nil, err
	}
	if blobSize >= 0 {
		if err := f.Truncate(blobSize); err != nil {
			return nil, err
		}
	}
	return f, nil
}

func (s *FSStore) Delete(_ context.Context, keys []string) error {
	for _, key := range keys {
		if err := os.Remove(filepath.Join(s.root, key)); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
	}
	return nil
}

func init() {
	var _ module.BlobStore = &FSStore{}
	module.Register((&FSStore{}).Name(), New)
}
