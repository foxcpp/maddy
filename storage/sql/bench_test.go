package sql

import (
	"flag"
	"math/rand"
	"strconv"
	"testing"
	"time"

	imapsql "github.com/foxcpp/go-imap-sql"
	"github.com/foxcpp/maddy/testutils"
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
		LazyUpdatesInit: true,
		CompressAlgo:    compAlgo,
	})
	if err != nil {
		tb.Fatal(err)
	}
	return &Storage{
		back:     db,
		hostname: "test.example.org",
	}
}

func BenchmarkStorage_Delivery(b *testing.B) {
	randomKey := "rcpt-" + strconv.FormatUint(rand.New(rand.NewSource(time.Now().UnixNano())).Uint64(), 10)

	be := createTestDB(b, "")
	if u, err := be.GetOrCreateUser(randomKey); err != nil {
		b.Fatal(err)
	} else {
		u.Logout()
	}

	testutils.BenchDelivery(b, be, "sender@example.org", []string{randomKey + "@example.org"})
}

func BenchmarkStorage_DeliveryLZ4(b *testing.B) {
	randomKey := "rcpt-" + strconv.FormatUint(rand.New(rand.NewSource(time.Now().UnixNano())).Uint64(), 10)

	be := createTestDB(b, "lz4")
	if u, err := be.GetOrCreateUser(randomKey); err != nil {
		b.Fatal(err)
	} else {
		u.Logout()
	}

	testutils.BenchDelivery(b, be, "sender@example.org", []string{randomKey + "@example.org"})
}

func BenchmarkStorage_DeliveryZstd(b *testing.B) {
	randomKey := "rcpt-" + strconv.FormatUint(rand.New(rand.NewSource(time.Now().UnixNano())).Uint64(), 10)

	be := createTestDB(b, "zstd")
	if u, err := be.GetOrCreateUser(randomKey); err != nil {
		b.Fatal(err)
	} else {
		u.Logout()
	}

	testutils.BenchDelivery(b, be, "sender@example.org", []string{randomKey + "@example.org"})
}
