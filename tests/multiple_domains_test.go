//go:build integration

/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2025 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

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

package tests_test

import (
	"testing"

	"github.com/foxcpp/maddy/tests"
)

// Test cases based on https://maddy.email/multiple-domains/

func TestMultipleDomains_SeparateNamespace(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)
	t.DNS(nil)
	t.Port("submission")
	t.Port("imap")
	t.Config(`
		tls off
		hostname test.maddy.email

		auth.pass_table local_authdb {
			table sql_table {
				driver sqlite3
				dsn credentials.db
				table_name passwords
			}
		}
		storage.imapsql local_mailboxes {
			driver sqlite3
			dsn imapsql.db
		}

		submission tcp://0.0.0.0:{env:TEST_PORT_submission} {
			auth &local_authdb
			reject
		}
		imap tcp://127.0.0.1:{env:TEST_PORT_imap} {
			auth &local_authdb
			storage &local_mailboxes
		}
	`)

	t.MustRunCLIGroup(
		[]string{"creds", "create", "-p", "user1", "user1@test1.maddy.email"},
		[]string{"creds", "create", "-p", "user2", "user2@test1.maddy.email"},
		[]string{"creds", "create", "-p", "user3", "user1@test2.maddy.email"},
		[]string{"imap-acct", "create", "--no-specialuse", "user1@test1.maddy.email"},
		[]string{"imap-acct", "create", "--no-specialuse", "user2@test1.maddy.email"},
		[]string{"imap-acct", "create", "--no-specialuse", "user1@test2.maddy.email"},
	)
	t.Run(2)

	user1 := t.Conn("imap")
	user1.ExpectPattern(`\* OK *`)
	user1.Writeln(`. LOGIN user1@test1.maddy.email user1`)
	user1.ExpectPattern(`. OK *`)
	user1.Writeln(`. CREATE user1`)
	user1.ExpectPattern(`. OK *`)

	user2 := t.Conn("imap")
	user2.ExpectPattern(`\* OK *`)
	user2.Writeln(`. LOGIN user2@test1.maddy.email user2`)
	user2.ExpectPattern(`. OK *`)
	user2.Writeln(`. CREATE user2`)
	user2.ExpectPattern(`. OK *`)

	user3 := t.Conn("imap")
	user3.ExpectPattern(`\* OK *`)
	user3.Writeln(`. LOGIN user1@test2.maddy.email user3`)
	user3.ExpectPattern(`. OK *`)
	user3.Writeln(`. CREATE user3`)
	user3.ExpectPattern(`. OK *`)

	user1.Writeln(`. LIST "" "*"`)
	user1.Expect(`* LIST (\HasNoChildren) "." INBOX`)
	user1.Expect(`* LIST (\HasNoChildren) "." "user1"`)
	user1.ExpectPattern(". OK *")

	user2.Writeln(`. LIST "" "*"`)
	user2.Expect(`* LIST (\HasNoChildren) "." INBOX`)
	user2.Expect(`* LIST (\HasNoChildren) "." "user2"`)
	user2.ExpectPattern(". OK *")

	user3.Writeln(`. LIST "" "*"`)
	user3.Expect(`* LIST (\HasNoChildren) "." INBOX`)
	user3.Expect(`* LIST (\HasNoChildren) "." "user3"`)
	user3.ExpectPattern(". OK *")
}

func TestMultipleDomains_SharedCredentials_DistinctMailboxes(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)
	t.DNS(nil)
	t.Port("submission")
	t.Port("imap")
	t.Config(`
		tls off
		hostname test.maddy.email
		auth_map email_localpart

		auth.pass_table local_authdb {
			table sql_table {
				driver sqlite3
				dsn credentials.db
				table_name passwords
			}
		}
		storage.imapsql local_mailboxes {
			driver sqlite3
			dsn imapsql.db
		}

		submission tcp://0.0.0.0:{env:TEST_PORT_submission} {
			auth &local_authdb
			reject
		}
		imap tcp://127.0.0.1:{env:TEST_PORT_imap} {
			auth &local_authdb
			storage &local_mailboxes
		}
	`)

	t.MustRunCLIGroup(
		[]string{"creds", "create", "-p", "user1", "user1"},
		[]string{"creds", "create", "-p", "user2", "user2"},
		[]string{"imap-acct", "create", "--no-specialuse", "user1@test1.maddy.email"},
		[]string{"imap-acct", "create", "--no-specialuse", "user2@test1.maddy.email"},
		[]string{"imap-acct", "create", "--no-specialuse", "user1@test2.maddy.email"},
	)
	t.Run(2)

	user1 := t.Conn("imap")
	user1.ExpectPattern(`\* OK *`)
	user1.Writeln(`. LOGIN user1@test1.maddy.email user1`)
	user1.ExpectPattern(`. OK *`)
	user1.Writeln(`. CREATE user1`)
	user1.ExpectPattern(`. OK *`)

	user2 := t.Conn("imap")
	user2.ExpectPattern(`\* OK *`)
	user2.Writeln(`. LOGIN user2@test1.maddy.email user2`)
	user2.ExpectPattern(`. OK *`)
	user2.Writeln(`. CREATE user2`)
	user2.ExpectPattern(`. OK *`)

	user3 := t.Conn("imap")
	user3.ExpectPattern(`\* OK *`)
	user3.Writeln(`. LOGIN user1@test2.maddy.email user1`)
	user3.ExpectPattern(`. OK *`)
	user3.Writeln(`. CREATE user3`)
	user3.ExpectPattern(`. OK *`)

	user1.Writeln(`. LIST "" "*"`)
	user1.Expect(`* LIST (\HasNoChildren) "." INBOX`)
	user1.Expect(`* LIST (\HasNoChildren) "." "user1"`)
	user1.ExpectPattern(". OK *")

	user2.Writeln(`. LIST "" "*"`)
	user2.Expect(`* LIST (\HasNoChildren) "." INBOX`)
	user2.Expect(`* LIST (\HasNoChildren) "." "user2"`)
	user2.ExpectPattern(". OK *")

	user3.Writeln(`. LIST "" "*"`)
	user3.Expect(`* LIST (\HasNoChildren) "." INBOX`)
	user3.Expect(`* LIST (\HasNoChildren) "." "user3"`)
	user3.ExpectPattern(". OK *")
}

func TestMultipleDomains_SharedCredentials_SharedMailboxes(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)
	t.DNS(nil)
	t.Port("submission")
	t.Port("imap")
	t.Config(`
		tls off
		hostname test.maddy.email
		auth_map email_localpart_optional

		auth.pass_table local_authdb {
			table sql_table {
				driver sqlite3
				dsn credentials.db
				table_name passwords
			}
		}
		storage.imapsql local_mailboxes {
			driver sqlite3
			dsn imapsql.db

			delivery_map email_localpart_optional
		}

		submission tcp://0.0.0.0:{env:TEST_PORT_submission} {
			auth &local_authdb
			reject
		}
		imap tcp://127.0.0.1:{env:TEST_PORT_imap} {
			auth &local_authdb
			storage &local_mailboxes

			storage_map email_localpart_optional
		}
	`)

	t.MustRunCLIGroup(
		[]string{"creds", "create", "-p", "user1", "user1"},
		[]string{"creds", "create", "-p", "user2", "user2"},
		[]string{"imap-acct", "create", "--no-specialuse", "user1"},
		[]string{"imap-acct", "create", "--no-specialuse", "user2"},
	)
	t.Run(2)

	user1 := t.Conn("imap")
	user1.ExpectPattern(`\* OK *`)
	user1.Writeln(`. LOGIN user1 user1`)
	user1.ExpectPattern(`. OK *`)
	user1.Writeln(`. CREATE user1`)
	user1.ExpectPattern(`. OK *`)

	user2 := t.Conn("imap")
	user2.ExpectPattern(`\* OK *`)
	user2.Writeln(`. LOGIN user2@test1.maddy.email user2`)
	user2.ExpectPattern(`. OK *`)
	user2.Writeln(`. CREATE user2`)
	user2.ExpectPattern(`. OK *`)

	user12 := t.Conn("imap")
	user12.ExpectPattern(`\* OK *`)
	user12.Writeln(`. LOGIN user1@test2.maddy.email user1`)
	user12.ExpectPattern(`. OK *`)
	user12.Writeln(`. CREATE user12`)
	user12.ExpectPattern(`. OK *`)

	user1.Writeln(`. LIST "" "*"`)
	user1.Expect(`* LIST (\HasNoChildren) "." INBOX`)
	user1.Expect(`* LIST (\HasNoChildren) "." "user1"`)
	user1.Expect(`* LIST (\HasNoChildren) "." "user12"`)
	user1.ExpectPattern(". OK *")

	user2.Writeln(`. LIST "" "*"`)
	user2.Expect(`* LIST (\HasNoChildren) "." INBOX`)
	user2.Expect(`* LIST (\HasNoChildren) "." "user2"`)
	user2.ExpectPattern(". OK *")

	user12.Writeln(`. LIST "" "*"`)
	user12.Expect(`* LIST (\HasNoChildren) "." INBOX`)
	user12.Expect(`* LIST (\HasNoChildren) "." "user1"`)
	user12.Expect(`* LIST (\HasNoChildren) "." "user12"`)
	user12.ExpectPattern(". OK *")
}
