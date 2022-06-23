//go:build cgo && !no_sqlite3
// +build cgo,!no_sqlite3

package blob

import (
	"math/rand"
	"testing"

	backendtests "github.com/foxcpp/go-imap-backend-tests"
	imapsql "github.com/foxcpp/go-imap-sql"
	"github.com/foxcpp/maddy/framework/module"
	imapsql2 "github.com/foxcpp/maddy/internal/storage/imapsql"
	"github.com/foxcpp/maddy/internal/testutils"
)

type testBack struct {
	backendtests.Backend
	ExtStore module.BlobStore
}

func TestStore(t *testing.T, newStore func() module.BlobStore, cleanStore func(module.BlobStore)) {
	// We use go-imap-sql backend and run a subset of
	// go-imap-backend-tests related to loading and saving messages.
	//
	// In the future we should probably switch to using a memory
	// backend for this.

	backendtests.Whitelist = []string{
		t.Name() + "/Mailbox_CreateMessage",
		t.Name() + "/Mailbox_ListMessages_Body",
		t.Name() + "/Mailbox_CopyMessages",
		t.Name() + "/Mailbox_Expunge",
		t.Name() + "/Mailbox_MoveMessages",
	}

	initBackend := func() backendtests.Backend {
		randSrc := rand.NewSource(0)
		prng := rand.New(randSrc)
		store := newStore()

		b, err := imapsql.New("sqlite3", ":memory:",
			imapsql2.ExtBlobStore{Base: store}, imapsql.Opts{
				PRNG: prng,
				Log:  testutils.Logger(t, "imapsql"),
			},
		)
		if err != nil {
			panic(err)
		}
		return testBack{Backend: b, ExtStore: store}
	}
	cleanBackend := func(bi backendtests.Backend) {
		b := bi.(testBack)
		if err := b.Backend.(*imapsql.Backend).Close(); err != nil {
			panic(err)
		}
		cleanStore(b.ExtStore)
	}

	backendtests.RunTests(t, initBackend, cleanBackend)
}
