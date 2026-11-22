// Package recordstest runs a records.Store through its contract, and picks the backend a store's
// own tests run against.
package recordstest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"cloud.google.com/go/datastore"
	"github.com/panyam/agni/internal/records"
	"github.com/panyam/agni/internal/records/gaerecords"
	"github.com/panyam/agni/internal/records/sqlrecords"
	"gorm.io/gorm"
)

// BackendEnv names the backend New and At return: "sqlite", "postgres", "datastore", or anything
// else for a folder. `make test-stores` runs the store packages' tests once with each (agni issue
// 1733).
const BackendEnv = "AGNI_TEST_RECORDS"

// PostgresEnv is the postgres:// URL the postgres backend connects to, with no password in it. The
// password comes from PGPASSWORD, as libpq's does. Each test dir gets a schema of its own there.
const PostgresEnv = "AGNI_TEST_POSTGRES"

// Backend is the backend BackendEnv names: file, sqlite, postgres or datastore.
func Backend() string {
	switch b := os.Getenv(BackendEnv); b {
	case "sqlite", "postgres", "datastore":
		return b
	}
	return "file"
}

// Folder reports whether the stores keep their records as files, the backend a test that reads a
// record off disk needs.
func Folder() bool { return Backend() == "file" }

// New returns an empty store over a fresh temp folder, in the backend BackendEnv names.
func New(t testing.TB) records.Store {
	t.Helper()
	return At(t, t.TempDir())
}

// At returns a store over dir in the backend BackendEnv names: the folder itself, a sqlite database
// in it, or a postgres schema or Datastore namespace named for it. Each call opens a new handle on
// the same records, as a server restarting does.
func At(t testing.TB, dir string) records.Store {
	t.Helper()
	switch Backend() {
	case "sqlite", "postgres":
		s, err := sqlrecords.NewSQL(DB(t, dir))
		if err != nil {
			t.Fatal(err)
		}
		return s
	case "datastore":
		c, ns := Datastore(t, dir)
		return gaerecords.New(c, ns)
	}
	d, err := records.NewDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// StoreURL is the --store URL of the records At(t, dir) opens: file://dir, sqlite://dir/.agni.db,
// AGNI_TEST_POSTGRES with dir's schema as its search_path, or datastore://<project>?namespace=dir's.
// It makes the postgres schema first.
func StoreURL(t testing.TB, dir string) string {
	t.Helper()
	slash := "/" + strings.TrimPrefix(filepath.ToSlash(dir), "/")
	switch Backend() {
	case "sqlite":
		return "sqlite://" + slash + "/.agni.db"
	case "postgres":
		return schemaURL(t, dir)
	case "datastore":
		return "datastore://" + project() + "?namespace=" + scope(dir)
	}
	return "file://" + slash
}

// scope names dir's postgres schema or Datastore namespace. Each test dir is fresh, so each test
// gets records of its own in one shared database.
func scope(dir string) string {
	h := sha256.Sum256([]byte(dir))
	return "t_" + hex.EncodeToString(h[:8])
}

// DB opens the sqlite database in dir, or the postgres schema named for dir, as BackendEnv says.
// The test closes it when it ends.
func DB(t testing.TB, dir string) *gorm.DB {
	t.Helper()
	var db *gorm.DB
	var err error
	if Backend() == "postgres" {
		db, err = sqlrecords.OpenPostgres(schemaURL(t, dir))
	} else {
		db, err = sqlrecords.OpenSQLite(filepath.Join(dir, ".agni.db"))
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	})
	return db
}

// schemaURL makes dir's schema in AGNI_TEST_POSTGRES and returns the URL that reads it, dropping the
// schema when the test ends.
func schemaURL(t testing.TB, dir string) string {
	t.Helper()
	base := os.Getenv(PostgresEnv)
	if base == "" {
		t.Fatalf("%s=postgres needs %s, a postgres:// URL, and PGPASSWORD", BackendEnv, PostgresEnv)
	}
	schema := scope(dir)
	admin, err := sqlrecords.OpenPostgres(base)
	if err != nil {
		t.Fatal(err)
	}
	if err := admin.Exec("CREATE SCHEMA IF NOT EXISTS " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		admin.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE")
		if sqlDB, err := admin.DB(); err == nil {
			sqlDB.Close()
		}
	})
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + "search_path=" + schema
}

// project is the Datastore project the datastore backend uses, DATASTORE_PROJECT_ID or agni-test.
func project() string {
	if p := os.Getenv("DATASTORE_PROJECT_ID"); p != "" {
		return p
	}
	return "agni-test"
}

// Datastore returns a client on DATASTORE_EMULATOR_HOST and the namespace named for dir. The test
// closes the client when it ends.
func Datastore(t testing.TB, dir string) (*datastore.Client, string) {
	t.Helper()
	if os.Getenv("DATASTORE_EMULATOR_HOST") == "" {
		t.Fatalf("%s=datastore needs DATASTORE_EMULATOR_HOST, a running emulator", BackendEnv)
	}
	c, err := datastore.NewClient(context.Background(), project())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c, scope(dir)
}

// Run checks the contract records.Store promises against stores newStore returns, each empty.
func Run(t *testing.T, newStore func(t *testing.T) records.Store) {
	ctx := context.Background()
	t.Run("GetPutDelete", func(t *testing.T) {
		s := newStore(t)
		if _, err := s.Get(ctx, "a/b.json"); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("Get of a missing key: %v, want fs.ErrNotExist", err)
		}
		if err := s.Put(ctx, "a/b.json", []byte("one")); err != nil {
			t.Fatal(err)
		}
		if err := s.Put(ctx, "a/b.json", []byte("two")); err != nil {
			t.Fatal(err)
		}
		if b, err := s.Get(ctx, "a/b.json"); err != nil || string(b) != "two" {
			t.Fatalf("Get = %q, %v; want two", b, err)
		}
		if err := s.Delete(ctx, "a/b.json"); err != nil {
			t.Fatal(err)
		}
		if err := s.Delete(ctx, "a/b.json"); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("second Delete: %v, want fs.ErrNotExist", err)
		}
		if ents, err := s.List(ctx, "a"); err != nil || len(ents) != 0 {
			t.Fatalf("List of an emptied folder = %v, %v; want none", ents, err)
		}
	})
	t.Run("Create", func(t *testing.T) {
		s := newStore(t)
		if err := s.Create(ctx, "r/x.json", []byte("first")); err != nil {
			t.Fatal(err)
		}
		if err := s.Create(ctx, "r/x.json", []byte("second")); !errors.Is(err, fs.ErrExist) {
			t.Fatalf("Create of a taken key: %v, want fs.ErrExist", err)
		}
		if b, _ := s.Get(ctx, "r/x.json"); string(b) != "first" {
			t.Fatalf("a refused Create changed the record to %q", b)
		}
	})
	t.Run("CreateRace", func(t *testing.T) {
		s := newStore(t)
		const n = 8
		var wg sync.WaitGroup
		errs := make([]error, n)
		for i := range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[i] = s.Create(ctx, "race.json", []byte(fmt.Sprint(i)))
			}()
		}
		wg.Wait()
		won := 0
		for _, err := range errs {
			switch {
			case err == nil:
				won++
			case !errors.Is(err, fs.ErrExist):
				t.Fatalf("a racing Create: %v", err)
			}
		}
		if won != 1 {
			t.Fatalf("%d of %d racing creates won, want 1", won, n)
		}
	})
	t.Run("List", func(t *testing.T) {
		s := newStore(t)
		// dx, d% and d_x catch a backend whose prefix match treats _ and % as wildcards.
		for _, k := range []string{"top.json", "d/b.json", "d/a.json", "d/sub/c.json", "d/sub/deeper/e.json", "d_x/f.json", "d0/g.json", "dx/h.json", "d%/p.json"} {
			if err := s.Put(ctx, k, []byte(k)); err != nil {
				t.Fatal(err)
			}
		}
		got, err := s.List(ctx, "d")
		if err != nil {
			t.Fatal(err)
		}
		want := []records.Entry{{Name: "a.json"}, {Name: "b.json"}, {Name: "sub", Dir: true}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("List(d) = %v, want %v", got, want)
		}
		top, err := s.List(ctx, "")
		if err != nil {
			t.Fatal(err)
		}
		wantTop := []records.Entry{{Name: "d", Dir: true}, {Name: "d%", Dir: true}, {Name: "d0", Dir: true}, {Name: "d_x", Dir: true}, {Name: "dx", Dir: true}, {Name: "top.json"}}
		if !reflect.DeepEqual(top, wantTop) {
			t.Fatalf("List() = %v, want %v", top, wantTop)
		}
		if got, err := s.List(ctx, "d_x"); err != nil || len(got) != 1 || got[0].Name != "f.json" {
			t.Fatalf("List(d_x) = %v, %v; want f.json alone, not dx's h.json", got, err)
		}
		if none, err := s.List(ctx, "never"); err != nil || len(none) != 0 {
			t.Fatalf("List of a folder never made = %v, %v; want none", none, err)
		}
	})
	t.Run("DeleteDir", func(t *testing.T) {
		s := newStore(t)
		for _, k := range []string{"d/a.json", "d/sub/c.json", "d0/keep.json", "dz.json", "d_/keep.json"} {
			if err := s.Put(ctx, k, nil); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.DeleteDir(ctx, "d"); err != nil {
			t.Fatal(err)
		}
		if err := s.DeleteDir(ctx, "never"); err != nil {
			t.Fatalf("DeleteDir of a folder never made: %v", err)
		}
		top, err := s.List(ctx, "")
		if err != nil {
			t.Fatal(err)
		}
		want := []records.Entry{{Name: "d0", Dir: true}, {Name: "d_", Dir: true}, {Name: "dz.json"}}
		if !reflect.DeepEqual(top, want) {
			t.Fatalf("after DeleteDir(d), List() = %v, want %v", top, want)
		}
	})
	t.Run("Case", func(t *testing.T) {
		s := newStore(t)
		if err := s.Put(ctx, "d/low.json", []byte("low")); err != nil {
			t.Fatal(err)
		}
		if err := s.Put(ctx, "D/up.json", []byte("up")); err != nil {
			t.Fatal(err)
		}
		if b, err := s.Get(ctx, "D/low.json"); err == nil && string(b) == "low" {
			// A folder on a filesystem that ignores case, such as macOS's default, holds one folder
			// for both. That is the filesystem's answer, not the store's.
			t.Skip("this store's filesystem ignores case")
		}
		if got, err := s.List(ctx, "d"); err != nil || len(got) != 1 || got[0].Name != "low.json" {
			t.Fatalf("List(d) = %v, %v; want low.json alone, not D's up.json", got, err)
		}
		if err := s.DeleteDir(ctx, "d"); err != nil {
			t.Fatal(err)
		}
		if b, err := s.Get(ctx, "D/up.json"); err != nil || string(b) != "up" {
			t.Fatalf("DeleteDir(d) took D/up.json: %q, %v", b, err)
		}
	})
	t.Run("Large", func(t *testing.T) {
		s := newStore(t)
		// Past Datastore's 1 MiB entity limit and gRPC's 4 MiB message, as a stored report on a real
		// board is.
		big := make([]byte, 9<<20+17)
		for i := range big {
			big[i] = byte(i % 251)
		}
		if err := s.Put(ctx, "r/big.json", big); err != nil {
			t.Fatal(err)
		}
		if got, err := s.Get(ctx, "r/big.json"); err != nil || !bytes.Equal(got, big) {
			t.Fatalf("Get of a %d-byte record = %d bytes, %v", len(big), len(got), err)
		}
		if err := s.Create(ctx, "r/big.json", big); !errors.Is(err, fs.ErrExist) {
			t.Fatalf("Create over a large record: %v, want fs.ErrExist", err)
		}
		if err := s.Put(ctx, "r/big.json", []byte("small")); err != nil {
			t.Fatal(err)
		}
		if got, err := s.Get(ctx, "r/big.json"); err != nil || string(got) != "small" {
			t.Fatalf("Get after shrinking = %q, %v", got, err)
		}
		if err := s.Delete(ctx, "r/big.json"); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("ReadReplace", func(t *testing.T) {
		s := newStore(t)
		if _, _, err := s.Read(ctx, "c/x.json"); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("Read of a missing key: %v, want fs.ErrNotExist", err)
		}
		if err := s.Put(ctx, "c/x.json", []byte("one")); err != nil {
			t.Fatal(err)
		}
		b, v1, err := s.Read(ctx, "c/x.json")
		if err != nil || string(b) != "one" {
			t.Fatalf("Read = %q, %v", b, err)
		}
		if err := s.Replace(ctx, "c/x.json", []byte("two"), v1); err != nil {
			t.Fatalf("Replace at the version Read saw: %v", err)
		}
		if err := s.Replace(ctx, "c/x.json", []byte("three"), v1); !errors.Is(err, records.ErrConflict) {
			t.Fatalf("Replace at a stale version: %v, want ErrConflict", err)
		}
		if got, _ := s.Get(ctx, "c/x.json"); string(got) != "two" {
			t.Fatalf("a refused Replace changed the record to %q", got)
		}
		_, v2, _ := s.Read(ctx, "c/x.json")
		if err := s.Put(ctx, "c/x.json", []byte("four")); err != nil {
			t.Fatal(err)
		}
		if err := s.Replace(ctx, "c/x.json", []byte("five"), v2); !errors.Is(err, records.ErrConflict) {
			t.Fatalf("Replace after a Put moved the record: %v, want ErrConflict", err)
		}
		_, v3, _ := s.Read(ctx, "c/x.json")
		if err := s.Delete(ctx, "c/x.json"); err != nil {
			t.Fatal(err)
		}
		if err := s.Replace(ctx, "c/x.json", []byte("six"), v3); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("Replace of a deleted record: %v, want fs.ErrNotExist", err)
		}
	})
	t.Run("UpdateRace", func(t *testing.T) {
		s := newStore(t)
		// Each writer adds one to a counter. A lost update leaves the count short.
		const n = 12
		var wg sync.WaitGroup
		errs := make([]error, n)
		for i := range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[i] = records.Update(ctx, s, "c/count", func(old []byte, exists bool) ([]byte, error) {
					c := 0
					if exists {
						fmt.Sscan(string(old), &c)
					}
					return []byte(fmt.Sprint(c + 1)), nil
				})
			}()
		}
		wg.Wait()
		for _, err := range errs {
			if err != nil {
				t.Fatalf("a racing Update: %v", err)
			}
		}
		if got, err := s.Get(ctx, "c/count"); err != nil || string(got) != fmt.Sprint(n) {
			t.Fatalf("count after %d racing Updates = %q, %v", n, got, err)
		}
	})
	t.Run("Sub", func(t *testing.T) {
		s := newStore(t)
		p := records.Sub(records.Sub(s, "outer"), "inner")
		if err := p.Put(ctx, "x.json", []byte("x")); err != nil {
			t.Fatal(err)
		}
		if b, err := s.Get(ctx, "outer/inner/x.json"); err != nil || string(b) != "x" {
			t.Fatalf("a Sub's record is at %q, %v", b, err)
		}
		if ents, err := p.List(ctx, ""); err != nil || len(ents) != 1 || ents[0].Name != "x.json" {
			t.Fatalf("Sub List = %v, %v", ents, err)
		}
		if err := p.DeleteDir(ctx, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Get(ctx, "outer/inner/x.json"); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("a Sub's DeleteDir left its record: %v", err)
		}
	})
	t.Run("BadKeys", func(t *testing.T) {
		s := newStore(t)
		for _, k := range []string{"", "/abs", "a/../b", "a//b", "a/", ".hidden", "a/.new-1", "..", `a\b`} {
			if err := s.Put(ctx, k, nil); !errors.Is(err, records.ErrInvalidKey) {
				t.Errorf("Put(%q): %v, want ErrInvalidKey", k, err)
			}
		}
		if _, err := records.Sub(s, "p").Get(ctx, "../escape"); !errors.Is(err, records.ErrInvalidKey) {
			t.Errorf("a Sub let ../escape through: %v", err)
		}
	})
}

// NewSQL returns a SQL store over the sqlite database at path. Two calls with one path share the
// database, as two servers would.
func NewSQL(t testing.TB, path string) *sqlrecords.SQL {
	t.Helper()
	db, err := sqlrecords.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	})
	s, err := sqlrecords.NewSQL(db)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
