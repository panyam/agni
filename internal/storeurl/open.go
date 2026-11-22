package storeurl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gormstore "github.com/panyam/goapplib/storage/backends/gorm"
	oagorm "github.com/panyam/oneauth/stores/gorm"
	"gorm.io/gorm"

	"github.com/panyam/agni/internal/authgate"
	"github.com/panyam/agni/internal/records"
	"github.com/panyam/agni/internal/records/sqlrecords"
	"github.com/panyam/agni/mountset"
)

// Store is an opened --store. A file:// store keeps each record as a file under Dir, in the layout
// every earlier agni wrote. A sqlite:// store keeps every record in DB: agni's own records in the
// agni_records table, runtime mounts and connections in goapplib's tables, and tokens in oneauth's.
// Neither holds file bytes, which live in mounts and --uploads.
type Store struct {
	// URL is the --store value it was opened from.
	URL string
	// Dir is a file:// store's folder, "" for sqlite.
	Dir string
	// DB is a sqlite:// store's database, nil for file://.
	DB *gorm.DB
	// Records holds agni's own records, each kind under a folder of its own, such as "designs" or
	// "reviews" (Sub).
	Records records.Store
}

// Open opens --store and reads --uploads. An empty store returns nil, and --uploads alone is
// refused, since uploads need a store to record what they hold. A sqlite store's database and its
// folder are created if absent.
func Open(store, uploads string) (*Store, string, error) {
	scheme, _, _ := strings.Cut(store, "://")
	if scheme != "sqlite" {
		dir, up, err := folders(store, uploads)
		if err != nil || dir == "" {
			return nil, up, err
		}
		recs, err := records.NewDir(dir)
		if err != nil {
			return nil, "", fmt.Errorf("--store %q: %w", store, err)
		}
		return &Store{URL: store, Dir: dir, Records: recs}, up, nil
	}
	path, err := sqlitePath(store)
	if err != nil {
		return nil, "", err
	}
	up, err := folderOf("uploads", uploads, uploadsSchemes)
	if err != nil {
		return nil, "", err
	}
	db, err := sqlrecords.OpenSQLite(path)
	if err != nil {
		return nil, "", fmt.Errorf("--store %q: %w", store, err)
	}
	recs, err := sqlrecords.NewSQL(db)
	if err != nil {
		closeDB(db)
		return nil, "", fmt.Errorf("--store %q: %w", store, err)
	}
	return &Store{URL: store, DB: db, Records: recs}, up, nil
}

// sqlitePath reads a sqlite:// URL, which names a database file by its absolute path:
// sqlite:///srv/agni.db. A relative path, a host, or a query is refused, since the server picks its
// own connection settings (sqlrecords.OpenSQLite).
func sqlitePath(store string) (string, error) {
	rest := strings.TrimPrefix(store, "sqlite://")
	if strings.ContainsAny(rest, "?#") {
		return "", fmt.Errorf("--store %q: a sqlite:// URL names a database file and takes no options; write it as sqlite:///srv/agni.db", store)
	}
	if !strings.HasPrefix(rest, "/") {
		return "", fmt.Errorf("--store %q names no absolute file; write it as sqlite:///srv/agni.db", store)
	}
	p := rest
	// A Windows path names its drive after the slash, sqlite:///C:/agni.db.
	if len(p) >= 3 && p[2] == ':' && filepath.VolumeName(p[1:]) != "" {
		p = p[1:]
	}
	p = filepath.Clean(filepath.FromSlash(p))
	if info, err := os.Stat(p); err == nil && info.IsDir() {
		return "", fmt.Errorf("--store %q names a folder; a sqlite:// URL names the database file in it, such as sqlite:///srv/agni/agni.db", store)
	}
	return p, nil
}

// Sub is the records of one kind, under the folder name.
func (s *Store) Sub(name string) records.Store { return records.Sub(s.Records, name) }

// Tokens is the store of the tokens `agni serve --auth tokens` checks.
func (s *Store) Tokens() (authgate.Keys, error) {
	if s.DB != nil {
		return authgate.SQLStore(s.DB)
	}
	return authgate.Store(s.Dir), nil
}

// TableStores returns the stores a mountset.Table keeps runtime mounts, connections and people's
// tokens in, for a sqlite store. A file:// store returns nil, and the table keeps them in folders
// under Dir as it always has.
func (s *Store) TableStores() (*mountset.Stores, error) {
	if s.DB == nil {
		return nil, nil
	}
	ms, err := gormstore.NewMountStore(s.DB)
	if err != nil {
		return nil, fmt.Errorf("--store %q: %w", s.URL, err)
	}
	cs, err := gormstore.NewConnectionStore(s.DB)
	if err != nil {
		return nil, fmt.Errorf("--store %q: %w", s.URL, err)
	}
	if err := s.DB.AutoMigrate(&oagorm.UserTokenModel{}); err != nil {
		return nil, fmt.Errorf("--store %q: %w", s.URL, err)
	}
	return &mountset.Stores{Mounts: ms, Connections: cs, UserTokens: oagorm.NewUserTokenStore(s.DB)}, nil
}

// Close closes a sqlite store's database. A file:// store holds nothing open.
func (s *Store) Close() error {
	if s == nil || s.DB == nil {
		return nil
	}
	return closeDB(s.DB)
}

func closeDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
