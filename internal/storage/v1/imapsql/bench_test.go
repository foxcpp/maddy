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

package imapsql

import (
	"flag"
	"strconv"
	"testing"
	"time"

	imapsql "github.com/foxcpp/go-imap-sql"
	"github.com/foxcpp/maddy/internal/testutils"
)

var (
	testDB      string
	testDSN     string
	testFsstore string
)

func init() {
	flag.StringVar(&testDB, "sql.testdb", "", "Database to use for storage/sql benchmarks")
	flag.StringVar(&testDSN, "sql.testdsn", "", "DSN to use for storage/sql benchmarks")
	flag.StringVar(&testFsstore, "sql.testfsstore", "", "fsstore location to use for storage/sql benchmarks")
}

func createTestDB(tb testing.TB, compAlgo string) *Storage {
	if testDB == "" || testDSN == "" || testFsstore == "" {
		tb.Skip("-sql.testdb, -sql.testdsn and -sql.testfsstore should be specified to run this benchmark")
	}

	db, err := imapsql.New(testDB, testDSN, &imapsql.FSStore{Root: testFsstore}, imapsql.Opts{
		CompressAlgo: compAlgo,
	})
	if err != nil {
		tb.Fatal(err)
	}
	return &Storage{
		Back: db,
	}
}

func BenchmarkStorage_Delivery(b *testing.B) {
	randomKey := "rcpt-" + strconv.FormatInt(time.Now().UnixNano(), 10) + "@example.org"

	be := createTestDB(b, "")
	if err := be.CreateIMAPAcct(randomKey); err != nil {
		b.Fatal(err)
	}

	testutils.BenchDelivery(b, be, "sender@example.org", []string{randomKey})
}

func BenchmarkStorage_DeliveryLZ4(b *testing.B) {
	randomKey := "rcpt-" + strconv.FormatInt(time.Now().UnixNano(), 10) + "@example.org"

	be := createTestDB(b, "lz4")
	if err := be.CreateIMAPAcct(randomKey); err != nil {
		b.Fatal(err)
	}

	testutils.BenchDelivery(b, be, "sender@example.org", []string{randomKey})
}

func BenchmarkStorage_DeliveryZstd(b *testing.B) {
	randomKey := "rcpt-" + strconv.FormatInt(time.Now().UnixNano(), 10) + "@example.org"

	be := createTestDB(b, "zstd")
	if err := be.CreateIMAPAcct(randomKey); err != nil {
		b.Fatal(err)
	}

	testutils.BenchDelivery(b, be, "sender@example.org", []string{randomKey})
}
