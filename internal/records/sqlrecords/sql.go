// Package sqlrecords keeps a server's records in a SQL database through GORM, for `--store
// sqlite://` and `--store postgres://` (agni issue 1733). It is apart from package records so that a program reading only
// folders never links a database driver.
package sqlrecords

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
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
	// Rev counts the row's writes. Replace writes only when it still holds the count Read saw.
	Rev int64 `gorm:"column:rev;not null;default:0"`
}

func (row) TableName() string { return "agni_records" }

// SQL is a Store over one table of a GORM database, `agni_records`, with a row per record keyed by
// its full key. A folder is the key prefix up to a slash, so it exists while a row sits under it.
// Each call is one statement, so several servers can share the database. Create inserts and lets
// the primary key refuse a taken key. Each write bumps the row's rev, and Replace updates only the
// row whose rev is still the one Read returned.
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
	r := row{Key: key, Body: body, Updated: s.now().UTC(), Rev: 1}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "record_key"}},
		DoUpdates: clause.Assignments(map[string]any{
			"body": body, "updated": r.Updated, "rev": gorm.Expr("agni_records.rev + 1"),
		}),
	}).Create(&r).Error
}

// Create inserts the row at key, doing nothing when key is taken.
func (s *SQL) Create(ctx context.Context, key string, body []byte) error {
	if err := records.CheckKey(key); err != nil {
		return err
	}
	r := row{Key: key, Body: body, Updated: s.now().UTC(), Rev: 1}
	res := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&r)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("record %s: %w", key, fs.ErrExist)
	}
	return nil
}

// Read reads the row at key with its rev.
func (s *SQL) Read(ctx context.Context, key string) ([]byte, records.Version, error) {
	if err := records.CheckKey(key); err != nil {
		return nil, "", err
	}
	var r row
	err := s.db.WithContext(ctx).Where("record_key = ?", key).Take(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", fmt.Errorf("record %s: %w", key, fs.ErrNotExist)
	}
	if err != nil {
		return nil, "", err
	}
	return r.Body, records.Version(strconv.FormatInt(r.Rev, 10)), nil
}

// Replace updates the row whose key is key and whose rev is still ver, in one statement. No row
// updated means the row moved on or went away, and a second read says which.
func (s *SQL) Replace(ctx context.Context, key string, body []byte, ver records.Version) error {
	if err := records.CheckKey(key); err != nil {
		return err
	}
	rev, err := strconv.ParseInt(string(ver), 10, 64)
	if err != nil {
		return fmt.Errorf("record %s: version %q is not one Read returned: %w", key, ver, records.ErrConflict)
	}
	res := s.db.WithContext(ctx).Model(&row{}).Where("record_key = ? AND rev = ?", key, rev).
		Updates(map[string]any{"body": body, "updated": s.now().UTC(), "rev": rev + 1})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 1 {
		return nil
	}
	if _, err := s.Get(ctx, key); err != nil {
		return err
	}
	return fmt.Errorf("record %s: %w", key, records.ErrConflict)
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

// under scopes a query to the keys that may sit below dir, with LIKE and an escaped prefix. LIKE
// ignores the column's collation, which a range scan over "dir/" to "dir0" would depend on, and
// postgres's default collations don't sort '/' before '0' byte by byte. sqlite's LIKE ignores ASCII
// case, so a caller keeps only the keys that start with the prefix exactly.
func under(q *gorm.DB, dir string) *gorm.DB {
	if dir == "" {
		return q
	}
	esc := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(dir + "/")
	return q.Where(`record_key LIKE ? ESCAPE '\'`, esc+"%")
}

// List reads the keys below dir and keeps the segment after dir of each.
func (s *SQL) List(ctx context.Context, dir string) ([]records.Entry, error) {
	if err := records.CheckDir(dir); err != nil {
		return nil, err
	}
	var keys []string
	if err := under(s.db.WithContext(ctx).Model(&row{}), dir).Pluck("record_key", &keys).Error; err != nil {
		return nil, err
	}
	return records.EntriesUnder(dir, keys), nil
}

// DeleteDir removes the rows below dir.
func (s *SQL) DeleteDir(ctx context.Context, dir string) error {
	if err := records.CheckDir(dir); err != nil {
		return err
	}
	if dir == "" {
		return s.db.WithContext(ctx).Where("1 = 1").Delete(&row{}).Error
	}
	// LIKE may match a key under the same folder in another case on sqlite, so the exact keys are
	// listed first and deleted by name.
	var keys []string
	if err := under(s.db.WithContext(ctx).Model(&row{}), dir).Pluck("record_key", &keys).Error; err != nil {
		return err
	}
	keys = records.KeysUnder(dir, keys)
	for len(keys) > 0 {
		n := min(len(keys), 500)
		if err := s.db.WithContext(ctx).Where("record_key IN ?", keys[:n]).Delete(&row{}).Error; err != nil {
			return err
		}
		keys = keys[n:]
	}
	return nil
}
