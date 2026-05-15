package fs

import (
	"os"
	"testing"

	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/storage/blob"
	"github.com/foxcpp/maddy/internal/testutils"
	"github.com/stretchr/testify/require"
)

func TestFS(t *testing.T) {
	blob.TestStore(t, func() module.BlobStore {
		dir := testutils.Dir(t)

		root, err := os.OpenRoot(dir)
		require.NoError(t, err)

		return &FSStore{
			instName: "test",
			rootPath: dir,
			root:     root,
		}
	}, func(store module.BlobStore) {
		require.NoError(t, os.RemoveAll(store.(*FSStore).rootPath))
	})
}
