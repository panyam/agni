package sqlrecords_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/panyam/agni/internal/records"
	"github.com/panyam/agni/internal/records/recordstest"
	"github.com/panyam/agni/internal/records/sqlrecords"
)

func TestSQLContract(t *testing.T) {
	recordstest.Run(t, func(t *testing.T) records.Store {
		return recordstest.NewSQL(t, filepath.Join(t.TempDir(), "agni.db"))
	})
}

// Two servers opening one database file see each other's records, and a create stays exclusive
// across them.
func TestSQLShared(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agni.db")
	a, b := recordstest.NewSQL(t, path), recordstest.NewSQL(t, path)
	ctx := context.Background()
	if err := a.Create(ctx, "p/x.json", []byte("from a")); err != nil {
		t.Fatal(err)
	}
	if got, err := b.Get(ctx, "p/x.json"); err != nil || string(got) != "from a" {
		t.Fatalf("b reads %q, %v", got, err)
	}
	if err := b.Create(ctx, "p/x.json", nil); err == nil {
		t.Fatal("b created a key a holds")
	}
}

// OpenSQLite's pragmas reach the driver. A DSN parameter the driver doesn't know is dropped
// silently, and concurrent writers then fail with "database is locked".
func TestOpenSQLitePragmas(t *testing.T) {
	dir := t.TempDir()
	db, err := sqlrecords.OpenSQLite(filepath.Join(dir, "sub", "agni.db"))
	if err != nil {
		t.Fatal(err)
	}
	// Closed before t.TempDir's cleanup, since Windows can't delete an open file.
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	})
	var mode string
	var busy int
	db.Raw("PRAGMA journal_mode").Scan(&mode)
	db.Raw("PRAGMA busy_timeout").Scan(&busy)
	if mode != "wal" || busy != 10000 {
		t.Fatalf("journal_mode %q, busy_timeout %d; want wal, 10000", mode, busy)
	}
	if err := db.Exec("CREATE TABLE t (x)").Error; err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		for _, f := range []string{"agni.db", "agni.db-wal"} {
			info, err := os.Stat(filepath.Join(dir, "sub", f))
			if err != nil {
				t.Fatal(err)
			}
			if m := info.Mode().Perm(); m != 0o600 {
				t.Errorf("%s has mode %v; want 0600, since it holds token hashes", f, m)
			}
		}
	}
}

// The same contract on postgres, when AGNI_TEST_POSTGRES names one (make test-stores starts it).
func TestPostgresContract(t *testing.T) {
	if os.Getenv(recordstest.PostgresEnv) == "" {
		t.Skipf("%s names no postgres; make test-stores runs this", recordstest.PostgresEnv)
	}
	t.Setenv(recordstest.BackendEnv, "postgres")
	recordstest.Run(t, func(t *testing.T) records.Store { return recordstest.New(t) })
}
