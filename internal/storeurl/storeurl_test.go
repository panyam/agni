package storeurl

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/panyam/agni/internal/records/recordstest"
	"github.com/panyam/oneauth/core"
)

func TestStoreAndUploadsTakeFileURLs(t *testing.T) {
	for _, tc := range []struct {
		store, uploads string
		wantStore      string
		wantUploads    string
		wantErr        string
	}{
		{store: "", uploads: ""},
		{store: "file:///srv/agni/meta", wantStore: "/srv/agni/meta"},
		{store: "file:///srv/agni/meta", uploads: "file:///srv/agni/files", wantStore: "/srv/agni/meta", wantUploads: "/srv/agni/files"},
		{store: "file://localhost/srv/agni/meta", wantStore: "/srv/agni/meta"},
		{store: "/srv/agni/meta", wantErr: "names a path with no scheme; write it as file:///"},
		{store: "mysql://db/agni", wantErr: "this build knows datastore://, file://, postgres://, postgresql://, sqlite://"},
		{store: "file://server/srv/agni", wantErr: "names the host"},
		{store: "file://relative", wantErr: "names the host"},
		{store: "file:///srv/agni/meta", uploads: "s3://bucket/agni", wantErr: "agni issue 1730"},
		{uploads: "file:///srv/agni/files", wantErr: "needs --store"},
	} {
		store, uploads, err := folders(tc.store, tc.uploads)
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("--store %q --uploads %q = %v, want an error naming %q", tc.store, tc.uploads, err, tc.wantErr)
			}
			continue
		}
		if err != nil || store != filepath.FromSlash(tc.wantStore) || uploads != filepath.FromSlash(tc.wantUploads) {
			t.Errorf("--store %q --uploads %q = %q, %q, %v; want %q, %q", tc.store, tc.uploads, store, uploads, err, tc.wantStore, tc.wantUploads)
		}
	}
	if runtime.GOOS == "windows" {
		if got, err := folderOf("store", "file:///C:/agni/meta", storeSchemes); err != nil || got != `C:\agni\meta` {
			t.Errorf("--store file:///C:/agni/meta = %q, %v; want C:\\agni\\meta", got, err)
		}
	}
}

func TestOpenTakesASqliteFile(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "sub", "agni.db")
	st, up, err := Open("sqlite://"+"/"+strings.TrimPrefix(filepath.ToSlash(db), "/"), "file://"+"/"+strings.TrimPrefix(filepath.ToSlash(dir), "/")+"/uploads")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if st.DB == nil || st.Dir != "" || up != filepath.Join(dir, "uploads") {
		t.Fatalf("Open = DB %v, Dir %q, uploads %q; want a database, no folder, and the uploads folder", st.DB, st.Dir, up)
	}
	if err := st.Sub("projects").Put(t.Context(), "p.json", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(db); err != nil {
		t.Fatalf("the database file: %v", err)
	}
	stores, err := st.TableStores()
	if err != nil || stores == nil || stores.Mounts == nil || stores.Connections == nil || stores.UserTokens == nil {
		t.Fatalf("TableStores = %+v, %v; want all three", stores, err)
	}
	if _, err := st.Tokens(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenRefusesABadSqliteURL(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct{ store, want string }{
		{"sqlite://agni.db", "names no absolute file"},
		{"sqlite:///srv/agni.db?cache=shared", "takes no options"},
		{"sqlite://" + "/" + strings.TrimPrefix(filepath.ToSlash(dir), "/"), "names a folder"},
	} {
		if _, _, err := Open(tc.store, ""); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Open(%q) = %v, want an error naming %q", tc.store, err, tc.want)
		}
	}
	if _, _, err := Open("", "file:///srv/files"); err == nil || !strings.Contains(err.Error(), "needs --store") {
		t.Errorf("--uploads alone = %v, want it refused", err)
	}
	if st, _, err := Open("", ""); err != nil || st != nil {
		t.Errorf("no --store = %v, %v; want nil and no error", st, err)
	}
}

func TestOpenKeepsAFileStoreAsFolders(t *testing.T) {
	dir := t.TempDir()
	st, _, err := Open("file://"+"/"+strings.TrimPrefix(filepath.ToSlash(dir), "/"), "")
	if err != nil {
		t.Fatal(err)
	}
	if st.Dir != dir || st.DB != nil {
		t.Fatalf("Open = Dir %q, DB %v; want the folder and no database", st.Dir, st.DB)
	}
	if stores, err := st.TableStores(); err != nil || stores != nil {
		t.Fatalf("a file store's TableStores = %v, %v; want nil, so the table keeps its folders", stores, err)
	}
	if err := st.Sub("projects").Put(t.Context(), "p.json", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "projects", "p.json")); err != nil {
		t.Fatalf("a file store keeps projects/p.json as a file: %v", err)
	}
}

func TestOpenRefusesABadPostgresOrDatastoreURL(t *testing.T) {
	for _, tc := range []struct{ store, want string }{
		{"postgres://agni:hunter2@db/agni", "names a password; put it in PGPASSWORD"},
		{"postgres://db", "names no host or database"},
		{"postgres:///agni", "names no host or database"},
		{"datastore://proj/extra", "datastore://<project>?namespace=<ns>"},
		{"datastore://proj?database=x", "takes namespace and nothing else"},
	} {
		_, _, err := Open(tc.store, "")
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Open(%q) = %v, want an error naming %q", tc.store, err, tc.want)
		}
		if err != nil && strings.Contains(err.Error(), "hunter2") {
			t.Errorf("Open(%q) put the password in its error: %v", tc.store, err)
		}
	}
}

// Open over a live postgres or Datastore emulator, when make test-stores starts one, keeps records
// and hands out the table's stores and the tokens.
func TestOpenALiveDatabaseOrDatastore(t *testing.T) {
	for name, u := range map[string]func(t *testing.T) string{
		"postgres": func(t *testing.T) string {
			if os.Getenv(recordstest.PostgresEnv) == "" {
				t.Skip("no postgres; make test-stores runs this")
			}
			t.Setenv(recordstest.BackendEnv, "postgres")
			return recordstest.StoreURL(t, t.TempDir())
		},
		"datastore": func(t *testing.T) string {
			if os.Getenv("DATASTORE_EMULATOR_HOST") == "" {
				t.Skip("no Datastore emulator; make test-stores runs this")
			}
			t.Setenv(recordstest.BackendEnv, "datastore")
			return recordstest.StoreURL(t, t.TempDir())
		},
	} {
		t.Run(name, func(t *testing.T) {
			st, _, err := Open(u(t), "")
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			if err := st.Sub("projects").Put(t.Context(), "p.json", []byte("{}")); err != nil {
				t.Fatal(err)
			}
			if b, err := st.Records.Get(t.Context(), "projects/p.json"); err != nil || string(b) != "{}" {
				t.Fatalf("Get = %q, %v", b, err)
			}
			stores, err := st.TableStores()
			if err != nil || stores == nil || stores.Mounts == nil || stores.Connections == nil || stores.UserTokens == nil {
				t.Fatalf("TableStores = %+v, %v; want all three", stores, err)
			}
			if _, err := stores.Mounts.ListMounts(t.Context()); err != nil {
				t.Fatalf("ListMounts on a fresh store: %v", err)
			}
			keys, err := st.Tokens()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := keys.ListSubjectAPIKeys(t.Context(), &core.ListSubjectAPIKeysRequest{Subject: "nobody"}); err != nil {
				t.Fatalf("tokens on a fresh store: %v", err)
			}
		})
	}
}
