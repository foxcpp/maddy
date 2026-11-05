package imapsql

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emersion/go-message/textproto"
	imapsql "github.com/foxcpp/go-imap-sql"
	"github.com/foxcpp/maddy/framework/exterrors"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func TestCommitStaleRecipientIsTemporary(t *testing.T) {
	dir := t.TempDir()
	storeDir := filepath.Join(dir, "messages")
	if err := os.Mkdir(storeDir, 0700); err != nil {
		t.Fatal(err)
	}

	back, err := imapsql.New("sqlite3", filepath.Join(dir, "test.db"), &imapsql.FSStore{Root: storeDir}, imapsql.Opts{BusyTimeout: 100})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { assert.NoError(t, back.Close()) }()

	if err := back.CreateUser("user"); err != nil {
		t.Fatal(err)
	}

	d := &delivery{d: back.NewDelivery()}
	if err := d.d.AddRcpt("user", textproto.Header{}); err != nil {
		t.Fatal(err)
	}
	if err := d.d.BodyRaw(strings.NewReader("Subject: test\r\n\r\nbody\r\n")); err != nil {
		t.Fatal(err)
	}

	if err := back.DeleteUser("user"); err != nil {
		t.Fatal(err)
	}

	err = d.Commit(context.Background())
	if code := exterrors.Fields(err)["smtp_code"]; code != 453 {
		t.Fatalf("commit error %v has SMTP code %v; want temporary 453", err, code)
	}
}

func TestCommitBusyDatabaseIsTemporary(t *testing.T) {
	dir := t.TempDir()
	storeDir := filepath.Join(dir, "messages")
	if err := os.Mkdir(storeDir, 0700); err != nil {
		t.Fatal(err)
	}

	back, err := imapsql.New("sqlite3", filepath.Join(dir, "test.db"), &imapsql.FSStore{Root: storeDir}, imapsql.Opts{BusyTimeout: 100})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { assert.NoError(t, back.Close()) }()

	if err := back.CreateUser("user"); err != nil {
		t.Fatal(err)
	}

	d := &delivery{d: back.NewDelivery()}
	if err := d.d.AddRcpt("user", textproto.Header{}); err != nil {
		t.Fatal(err)
	}
	if err := d.d.BodyRaw(strings.NewReader("Subject: test\r\n\r\nbody\r\n")); err != nil {
		t.Fatal(err)
	}

	tx, err := back.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec("ROLLBACK; BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	defer func() { assert.NoError(t, tx.Rollback()) }()

	err = d.Commit(context.Background())
	if code := exterrors.Fields(err)["smtp_code"]; code != 453 {
		t.Fatalf("busy commit error %v has SMTP code %v; want temporary 453", err, code)
	}
}
