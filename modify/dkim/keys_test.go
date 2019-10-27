package dkim

import (
	"crypto/ed25519"
	"encoding/base64"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/foxcpp/maddy/testutils"
)

func TestKeyLoad_new(t *testing.T) {
	m := Modifier{}
	m.log = testutils.Logger(t, m.Name())

	dir, err := ioutil.TempDir("", "maddy-tests-dkim-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	signer, err := m.loadOrGenerateKey("example.org", "default", filepath.Join(dir, "testkey.key"), "ed25519")
	if err != nil {
		t.Fatal(err)
	}

	recordBlob, err := ioutil.ReadFile(filepath.Join(dir, "testkey.dns"))
	if err != nil {
		t.Fatal(err)
	}
	var keyBlob []byte
	for _, part := range strings.Split(string(recordBlob), ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "k=") {
			if part != "k=ed25519" {
				t.Fatalf("Wrong type of generated key, want ed25519, got %s", part)
			}
		}
		if strings.HasPrefix(part, "p=") {
			keyBlob, err = base64.StdEncoding.DecodeString(part[2:])
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	blob := signer.Public().(ed25519.PublicKey)
	if string(blob) != string(keyBlob) {
		t.Fatal("wrong public key placed into record file")
	}
}

const pkeyEd25519 = `-----BEGIN PRIVATE KEY-----
X-DKIM-Domain: example.org
X-DKIM-Selector: default

MC4CAQAwBQYDK2VwBCIEIJG9zs4vi2MYNkL9gUQwlmBLCzDODIJ5/1CwTAZFDm5U
-----END PRIVATE KEY-----`

const pubkeyEd25519 = `5TPcCxzVByMyRsMFs5Dx23pnxKilI+1UrGg0t+O2oZU=`

func TestKeyLoad_existing(t *testing.T) {
	m := Modifier{}
	m.log = testutils.Logger(t, m.Name())

	dir, err := ioutil.TempDir("", "maddy-tests-dkim-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	if err := ioutil.WriteFile(filepath.Join(dir, "testkey.key"), []byte(pkeyEd25519), 0600); err != nil {
		t.Fatal(err)
	}

	signer, err := m.loadOrGenerateKey("example.org", "default", filepath.Join(dir, "testkey.key"), "ed25519")
	if err != nil {
		t.Fatal(err)
	}

	blob := signer.Public().(ed25519.PublicKey)
	if signerKey := base64.StdEncoding.EncodeToString(blob); signerKey != pubkeyEd25519 {
		t.Fatalf("wrong public key returned by loadOrGenerateKey, \nwant %s\ngot  %s", pubkeyEd25519, signerKey)
	}
}
