//go:build !cgo || no_sqlite3
// +build !cgo no_sqlite3

package blob

import (
	"testing"

	"github.com/foxcpp/maddy/framework/module"
)

func TestStore(t *testing.T, newStore func() module.BlobStore, cleanStore func(module.BlobStore)) {
	t.Skip("storage.blob tests require CGo and sqlite3")
}
