/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package dkim

import (
	"crypto/ed25519"
	"crypto/rsa"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/foxcpp/maddy/internal/testutils"
)

func TestKeyLoad_new(t *testing.T) {
	m := Modifier{}
	m.log = testutils.Logger(t, m.Name())

	dir := t.TempDir()

	signer, newKey, err := m.loadOrGenerateKey("", filepath.Join(dir, "testkey.key"), "ed25519", false)
	if err != nil {
		t.Fatal(err)
	}
	if !newKey {
		t.Fatal("newKey=false")
	}

	recordBlob, err := os.ReadFile(filepath.Join(dir, "testkey.dns"))
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
MC4CAQAwBQYDK2VwBCIEIJG9zs4vi2MYNkL9gUQwlmBLCzDODIJ5/1CwTAZFDm5U
-----END PRIVATE KEY-----`

const pubkeyEd25519 = `5TPcCxzVByMyRsMFs5Dx23pnxKilI+1UrGg0t+O2oZU=`

func TestKeyLoad_existing_pkcs8(t *testing.T) {
	m := Modifier{}
	m.log = testutils.Logger(t, m.Name())

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "testkey.key"), []byte(pkeyEd25519), 0o600); err != nil {
		t.Fatal(err)
	}

	signer, newKey, err := m.loadOrGenerateKey("", filepath.Join(dir, "testkey.key"), "ed25519", false)
	if err != nil {
		t.Fatal(err)
	}
	if newKey {
		t.Fatal("newKey = true")
	}

	blob := signer.Public().(ed25519.PublicKey)
	if signerKey := base64.StdEncoding.EncodeToString(blob); signerKey != pubkeyEd25519 {
		t.Fatalf("wrong public key returned by loadOrGenerateKey, \nwant %s\ngot  %s", pubkeyEd25519, signerKey)
	}
}

const pkeyRSA = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAuxWwDR9ADiuV2b9xF+btOIgwS5W0yJeS/Dht4HlUELrye2JZ
7TCQpx2Hs1FY5Tkj4VLnYHTPftS6cLYNx6hQbWZMhj5qmP9ccQ8rqdgdLB5RqCn3
zo8wbKFZ8ygYt1yZyNOfJLNTBjIcC1BCKoZosA7MWHUOwRtt1ARVmldsNH3iio0l
wHjyKNYd0Kqw4uGEg6sulK69lw4G8YTnKtCt0G8vCpQHyQepolOMF7Q1NZEw02/U
E54qgaaC+ym+BQsqqF5iodmuIfLX+W0kKDee2YYhjuxNaFcPhE5j35LlGHCsrL0X
h4+2VZSYXuAO5aWpwX9jrrSFyCJLD/aYGMgdrwIDAQABAoIBAEZrF2UZCidLSJA5
evwgM9I/kM4if3Wxd+Xv54vCn13cwECo+GhLC2ebueRJDkjZhSPe7LBlx2RZ9gNO
w0kPlZZYFx3AiKcmF0mHCExZyEE++EVv5pKdWwDIiu73fLYn6MqqvRA3X1zJp7yq
bP1MskLyjwAMr40IIgLXztDVbykiRC2Rw+o5cu7o3e0p0sFqJsjCUKtXZuzLePOk
gYYZ4FsmmVYh7pf244NEQao+fT19RtFL85E17yAHv+YD7qUBdbxoWIuAher9N/C0
vOj4xYbNxbkS0+BTbygLAog5mFtNbAGysUZZ3YOYfKYgj9/u+aKwr2ZS2zIEeJj0
eAiHtWECgYEA48dqxrR76JyukHid+XyI4Nqt+2EHEeDi23WTTT6lSZL1F3I2q7FF
DSHOA3hGw57GAMNQYCSzYxC4TBpZwJ7/8NdhA/kJg7tLOqcvZtS3Bu5bzLqLOCqL
E1tgh2LrpWjit2v+VSsQlf+QjG7QAEiWtya+AOfNWenILfxk2VNPP3MCgYEA0kOM
ym/EcgcSSihbFyyYO4UHZZ7rWiPRB+BtatJbEADMXMlwSAXvvVCpWSZBKBKjIE2y
ZM+kvv50QUd4ue7dKVEnqOy26XuAmuTE14smx1QyNonRvBV/HItJ0tKfMIZbXOpq
S2ESXkFybCzdOfzWOhx0PHjr40w8XUeSZi0LodUCgYAsC8bhD8uaKpozA7AAq41I
deEI6DVWxrb3mx/V4xRRSuKsGwDpaIkixfOxhhOhBlXhleM4BEDQGk6ZIMtUTSrO
5scy3nhxick9WVD4QI/3/iWwTC5ZuRhVsOjUpVNOFB8rOu3eiEpXxyirj04Xj/Hd
DtfVEv4JsgRsqA7UW6DKcwKBgQCiCvMXFDnWEwMSabWBz5lmzWfc9jO1HUM8Ccbp
e0I4vBTDMW854nFXejF5BhVS18Il5BsmvCvgEePwZy9wQ9jnvaaN9hglKkv7k3Ds
GE6DcazdASvFAuAaVHJJao7Ka9E/c10FyMLKJzASlCTOSr+iu0kNTbelTZx72uvF
mNONHQKBgCEUuJMM11mV0FCsVfJsmIv6z/zqOiPiOVbP1Bv2WlVzipvkI9bm6OyN
VHO8+oqFWyhJ3qRzebuPIefL8U6xjfMshX8MB23cB0J5LTPDZH3LCSmFvjr942EK
5+ewYHKtmS+6aaE+J+oB11r7XU8FyEI0kv6rAPDwJ19K4BMG/x7J
-----END RSA PRIVATE KEY-----`

func TestKeyLoad_existing_pkcs1(t *testing.T) {
	m := Modifier{}
	m.log = testutils.Logger(t, m.Name())

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "testkey.key"), []byte(pkeyRSA), 0o600); err != nil {
		t.Fatal(err)
	}

	signer, newKey, err := m.loadOrGenerateKey("", filepath.Join(dir, "testkey.key"), "rsa2048", false)
	if err != nil {
		t.Fatal(err)
	}
	if newKey {
		t.Fatal("newKey=true")
	}

	pubkey := signer.Public().(*rsa.PublicKey)
	if pubkey.E != 65537 {
		t.Fatalf("wrong public key returned by loadOrGenerateKey, got %d", pubkey.E)
	}
	if pubkey.N.String() != "23617257632228188386824425094266725423560758883229529475904285522114491665694237598874002862630696077162868821164059728985148713872807170386818903503533709975391952347175641552635505497204925274569104682448177717429244936284920784061388978739927939000424446717818401440783667723710780854637197555911253613285419663410256437304926940168312631109994734698918250930969511949067760562140706765511288141008942649676427142664185811322596443990204153105455693515405445788622172538582060141770589195075185467867938584021491237815987395835392935511032761463924045865609068314478096903374718657496007822964380498648030935260591" {
		t.Fatalf("wrong public key returned by loadOrGenerateKey, got %s", pubkey.N.String())
	}
}
