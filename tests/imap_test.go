//go:build integration && cgo && !nosqlite3
// +build integration,cgo,!nosqlite3

package tests_test

import (
	"testing"

	"github.com/foxcpp/maddy/tests"
)

func TestIMAPEndpointAuthMap(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)

	t.DNS(nil)
	t.Port("imap")
	t.Config(`
		storage.imapsql test_store {
			driver sqlite3
			dsn imapsql.db
		}

		imap tcp://127.0.0.1:{env:TEST_PORT_imap} {
			tls off

			auth_map email_localpart
			auth pass_table static {
				entry "user" "bcrypt:$2a$10$E.AuCH3oYbaRrETXfXwc0.4jRAQBbanpZiCfudsJz9bHzLr/qj6ti" # password: 123
			}
			storage &test_store
		}
	`)
	t.Run(1)
	defer t.Close()

	imapConn := t.Conn("imap")
	defer imapConn.Close()
	imapConn.ExpectPattern(`\* OK *`)
	imapConn.Writeln(". LOGIN user@example.org 123")
	imapConn.ExpectPattern(". OK *")
	imapConn.Writeln(". SELECT INBOX")
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`. OK *`)
}

func TestIMAPEndpointStorageMap(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)

	t.DNS(nil)
	t.Port("imap")
	t.Config(`
		storage.imapsql test_store {
			driver sqlite3
			dsn imapsql.db
		}

		imap tcp://127.0.0.1:{env:TEST_PORT_imap} {
			tls off

			storage_map email_localpart

			auth_map email_localpart
			auth pass_table static {
				entry "user" "bcrypt:$2a$10$z9SvUwUjkY8wKOWd9IbISeEmbJua2cXRPqw7s2BnLXJuc6pIMPncK" # password: 123
			}
			storage &test_store
		}
	`)
	t.Run(1)
	defer t.Close()

	imapConn := t.Conn("imap")
	defer imapConn.Close()
	imapConn.ExpectPattern(`\* OK *`)
	imapConn.Writeln(". LOGIN user@example.org 123")
	imapConn.ExpectPattern(". OK *")
	imapConn.Writeln(". CREATE testbox")
	imapConn.ExpectPattern(". OK *")

	imapConn2 := t.Conn("imap")
	defer imapConn2.Close()
	imapConn2.ExpectPattern(`\* OK *`)
	imapConn2.Writeln(". LOGIN user@example.com 123")
	imapConn2.ExpectPattern(". OK *")
	imapConn2.Writeln(`. LIST "" "*"`)
	imapConn2.Expect(`* LIST (\HasNoChildren) "." INBOX`)
	imapConn2.Expect(`* LIST (\HasNoChildren) "." "testbox"`)
	imapConn2.ExpectPattern(". OK *")
}
