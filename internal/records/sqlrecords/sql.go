// Package sqlrecords keeps a server's records in a SQL database through GORM, for `--store
// sqlite://` (agni issue 1733). It is apart from package records so that a program reading only
// folders never links a database driver.
package sqlrecords

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/panyam/agni/internal/records"
)

// row is one record in the agni_records table. Its column names are the schema, so renaming a Go
// field doesn't change the table.
type row struct {
	Key     string    `gorm:"column:record_key;primaryKey;size:512"`
	Body    []byte    `gorm:"column:body"`
	Updated time.Time `gorm:"column:updated"`
}

func (row) TableName() string { return "agni_records" }

// SQL is a Store over one table of a GORM database, `agni_records`, with a row per record keyed by
// its full key. A folder is the key prefix up to a slash, so it exists while a row sits under it.
// Each call is one statement, so several servers can share the database. Create inserts and lets
// the primary key refuse a taken key.
type SQL struct {
	db  *gorm.DB
	now func() time.Time
}

// NewSQL returns a Store over db, creating its table if absent.
func NewSQL(db *gorm.DB) (*SQL, error) {
	if err := db.AutoMigrate(&row{}); err != nil {
		return nil, fmt.Errorf("records: %w", err)
	}
	return &SQL{db: db, now: time.Now}, nil
}

var _ records.Store = (*SQL)(nil)

// Get reads the row at key.
func (s *SQL) Get(ctx context.Context, key string) ([]byte, error) {
	if err := records.CheckKey(key); err != nil {
		return nil, err
	}
	var r row
	err := s.db.WithContext(ctx).Where("record_key = ?", key).Take(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("record %s: %w", key, fs.ErrNotExist)
	}
	if err != nil {
		return nil, err
	}
	return r.Body, nil
}

// Put upserts the row at key.
func (s *SQL) Put(ctx context.Context, key string, body []byte) error {
	if err := records.CheckKey(key); err != nil {
		return err
	}
	r := row{Key: key, Body: body, Updated: s.now().UTC()}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(&r).Error
}

// Create inserts the row at key, doing nothing when key is taken.
func (s *SQL) Create(ctx context.Context, key string, body []byte) error {
	if err := records.CheckKey(key); err != nil {
		return err
	}
	r := row{Key: key, Body: body, Updated: s.now().UTC()}
	res := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&r)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("record %s: %w", key, fs.ErrExist)
	}
	return nil
}

// Delete removes the row at key.
func (s *SQL) Delete(ctx context.Context, key string) error {
	if err := records.CheckKey(key); err != nil {
		return err
	}
	res := s.db.WithContext(ctx).Where("record_key = ?", key).Delete(&row{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("record %s: %w", key, fs.ErrNotExist)
	}
	return nil
}

// under scopes a query to the keys below dir. It compares ranges rather than using LIKE, so an id
// holding '_' or '%' needs no escaping: every key under "a/b" sorts from "a/b/" up to "a/b0", since
// '0' follows '/'.
func under(q *gorm.DB, dir string) *gorm.DB {
	if dir == "" {
		return q
	}
	return q.Where("record_key >= ? AND record_key < ?", dir+"/", dir+"0")
}

// List reads the keys below dir and keeps the segment after dir of each.
func (s *SQL) List(ctx context.Context, dir string) ([]records.Entry, error) {
	if err := checkDir(dir); err != nil {
		return nil, err
	}
	var keys []string
	if err := under(s.db.WithContext(ctx).Model(&row{}), dir).Order("record_key").Pluck("record_key", &keys).Error; err != nil {
		return nil, err
	}
	prefix := ""
	if dir != "" {
		prefix = dir + "/"
	}
	var out []records.Entry
	seen := map[string]bool{}
	for _, k := range keys {
		rest := strings.TrimPrefix(k, prefix)
		name, _, isDir := strings.Cut(rest, "/")
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, records.Entry{Name: name, Dir: isDir})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// checkDir is records.CheckKey for a folder, where "" names the top.
func checkDir(dir string) error {
	if dir == "" {
		return nil
	}
	return records.CheckKey(dir)
}

// DeleteDir removes the rows below dir.
func (s *SQL) DeleteDir(ctx context.Context, dir string) error {
	if err := checkDir(dir); err != nil {
		return err
	}
	q := s.db.WithContext(ctx)
	if dir == "" {
		q = q.Where("1 = 1")
	}
	return under(q, dir).Delete(&row{}).Error
}
