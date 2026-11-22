package sqlrecords

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// OpenSQLite opens the sqlite database at path, creating it (mode 0600) and its folder if absent. It uses a
// pure-Go driver, since agni builds with CGO_ENABLED=0. The database runs in WAL mode, so readers
// don't wait on a writer, and takes its write lock at BEGIN (_txlock=immediate) with a ten-second
// busy timeout, so two writers queue rather than fail with "database is locked". goapplib's GORM
// store needs the same (its storage/CLAUDE.md).
func OpenSQLite(path string) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	// The database holds connection records, sealed tokens and token hashes, so a new one is the
	// owner's alone, as the folder store's files are. SQLite gives its -wal and -shm files the
	// database's mode.
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600) // #nosec G304 -- the operator's --store path
	if err != nil {
		return nil, err
	}
	f.Close()
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(10000)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Set("_txlock", "immediate")
	db, err := gorm.Open(sqlite.Open("file:"+path+"?"+q.Encode()), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return nil, fmt.Errorf("sqlite %s: %w", path, err)
	}
	return db, nil
}

// OpenPostgres opens the postgres database dsn names, a postgres:// URL or a key=value string as
// pgx reads them. The password comes from PGPASSWORD or a passfile, as libpq's do, so it never
// sits on a command line. The pure-Go pgx driver needs no cgo either.
func OpenPostgres(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}
	return db, nil
}
