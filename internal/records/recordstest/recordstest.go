// Package recordstest runs a records.Store through its contract, and picks the backend a store's
// own tests run against.
package recordstest

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/panyam/agni/internal/records"
	"github.com/panyam/agni/internal/records/sqlrecords"
)

// BackendEnv names the backend New returns: "sqlite", or anything else for a folder. `make
// testall` runs the store packages' tests once with each (agni issue 1733).
const BackendEnv = "AGNI_TEST_RECORDS"

// SQLite reports whether BackendEnv asks for sqlite.
func SQLite() bool { return os.Getenv(BackendEnv) == "sqlite" }

// New returns an empty store in a fresh temp folder, kept as files or, with BackendEnv=sqlite, as
// rows of a sqlite database in that folder.
func New(t testing.TB) records.Store {
	t.Helper()
	if SQLite() {
		return NewSQL(t, filepath.Join(t.TempDir(), "agni.db"))
	}
	d, err := records.NewDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// At returns a store over dir: the folder itself, or, with BackendEnv=sqlite, a sqlite database in
// dir. Each call opens a new handle on the same records, as a server restarting does.
func At(t testing.TB, dir string) records.Store {
	t.Helper()
	if SQLite() {
		return NewSQL(t, filepath.Join(dir, ".agni.db"))
	}
	d, err := records.NewDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return d
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
		for _, k := range []string{"top.json", "d/b.json", "d/a.json", "d/sub/c.json", "d/sub/deeper/e.json", "d_x/f.json", "d0/g.json"} {
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
		wantTop := []records.Entry{{Name: "d", Dir: true}, {Name: "d0", Dir: true}, {Name: "d_x", Dir: true}, {Name: "top.json"}}
		if !reflect.DeepEqual(top, wantTop) {
			t.Fatalf("List() = %v, want %v", top, wantTop)
		}
		if none, err := s.List(ctx, "never"); err != nil || len(none) != 0 {
			t.Fatalf("List of a folder never made = %v, %v; want none", none, err)
		}
	})
	t.Run("DeleteDir", func(t *testing.T) {
		s := newStore(t)
		for _, k := range []string{"d/a.json", "d/sub/c.json", "d0/keep.json", "dz.json"} {
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
		want := []records.Entry{{Name: "d0", Dir: true}, {Name: "dz.json"}}
		if !reflect.DeepEqual(top, want) {
			t.Fatalf("after DeleteDir(d), List() = %v, want %v", top, want)
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
