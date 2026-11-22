// Package records keeps the small records a server writes under --store (agni issue 1733). Each
// record is a few kilobytes of JSON or YAML under a slash-separated key, such as
// "projects/board.json" or "reports/jetson/r1.json". Dir keeps each record as a file, the layout
// `--store file://` has always had. sqlrecords.SQL keeps them as rows of one table, for
// `--store sqlite://`.
//
// A store holds no file bytes. A design's files live in mounts and the uploads' blobs.
package records

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"
)

// Store keeps records by key. A key is a clean slash path: no leading or trailing slash, no empty,
// "." or ".." segment, and no segment that starts with a dot, which Dir keeps for its temp files.
// Every call refuses any other key with ErrInvalidKey.
//
// A missing record is fs.ErrNotExist and a taken key on Create is fs.ErrExist, so a caller tests both
// with errors.Is as it did against the OS. Each write replaces a record whole, so a reader sees the
// old record or the new one and never a mix.
//
// A read-modify-write goes through Read and Replace, or Update around them. Replace writes only when
// the record is still at the version Read saw, across processes sharing the store, so two servers
// editing one record can't lose either edit (agni issue 1823).
type Store interface {
	// Get returns the record at key, or fs.ErrNotExist.
	Get(ctx context.Context, key string) ([]byte, error)
	// Put writes the record at key, replacing any it had.
	Put(ctx context.Context, key string, body []byte) error
	// Create writes the record at key, or returns fs.ErrExist when key holds one. Two creates racing
	// for one key leave exactly one record and one fs.ErrExist.
	Create(ctx context.Context, key string, body []byte) error
	// Delete removes the record at key, or returns fs.ErrNotExist.
	Delete(ctx context.Context, key string) error
	// List returns what sits directly under dir, ordered by name: each record, and each folder that
	// holds records deeper down. "" lists the top. A dir holding nothing returns no entries and no
	// error, as a folder that was never made.
	List(ctx context.Context, dir string) ([]Entry, error)
	// DeleteDir removes every record under dir. A dir holding nothing is no error.
	DeleteDir(ctx context.Context, dir string) error
	// Read returns the record at key and its version, or fs.ErrNotExist. The version changes with
	// every write that changes the record, Put included.
	Read(ctx context.Context, key string) ([]byte, Version, error)
	// Replace writes the record at key if it is still at version ver. A record written since is
	// ErrConflict, and one deleted since is fs.ErrNotExist.
	Replace(ctx context.Context, key string, body []byte, ver Version) error
}

// Sweeper is a Store that leaves garbage a crashed writer can strand, such as Datastore's parts of a
// large record, and removes it when asked. A server calls Sweep beside its uploads sweep. grace
// spares a write still in flight. It returns how much it removed.
type Sweeper interface {
	Sweep(ctx context.Context, grace time.Duration) (int, error)
}

// Version names one write of a record, as Read returns it. Only the backend that returned it reads
// it.
type Version string

// ErrConflict is a Replace whose record changed after the Read it names.
var ErrConflict = errors.New("records: the record changed since it was read")

// updateAttempts is how often Update retries a write another writer beat.
const updateAttempts = 32

// Update reads the record at key, hands it to fn, and writes what fn returns, all as one change: a
// record another writer changed in between is read again and fn runs again, up to updateAttempts
// times. exists says whether the record was there. fn returning a nil body with no error deletes
// nothing and writes nothing. An error from fn stops Update and comes back as it is. fn may run more
// than once, so it must not keep side effects from a run whose write lost.
func Update(ctx context.Context, s Store, key string, fn func(old []byte, exists bool) ([]byte, error)) error {
	for range updateAttempts {
		old, ver, err := s.Read(ctx, key)
		exists := err == nil
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		body, err := fn(old, exists)
		if err != nil || body == nil {
			return err
		}
		if exists {
			err = s.Replace(ctx, key, body, ver)
		} else {
			err = s.Create(ctx, key, body)
		}
		if errors.Is(err, ErrConflict) || errors.Is(err, fs.ErrExist) || (exists && errors.Is(err, fs.ErrNotExist)) {
			continue
		}
		return err
	}
	return fmt.Errorf("record %s: %w after %d attempts", key, ErrConflict, updateAttempts)
}

// Entry is one name List found under a folder.
type Entry struct {
	// Name is the last segment of the key, such as "board.json", or a folder's name.
	Name string
	// Dir is true for a folder that holds records, and false for a record.
	Dir bool
}

// sortEntries orders entries by name, as List promises.
func sortEntries(es []Entry) { sort.Slice(es, func(i, j int) bool { return es[i].Name < es[j].Name }) }

// ErrInvalidKey is a key that is not a clean slash path, or that names a dot segment.
var ErrInvalidKey = fs.ErrInvalid

// CheckKey returns ErrInvalidKey, wrapped with key, unless key is a clean slash path whose segments
// are neither empty nor start with a dot.
func CheckKey(key string) error {
	if key == "" || path.Clean(key) != key || strings.HasPrefix(key, "/") {
		return fmt.Errorf("%w: record key %q", ErrInvalidKey, key)
	}
	for _, seg := range strings.Split(key, "/") {
		if strings.HasPrefix(seg, ".") || strings.ContainsRune(seg, '\\') {
			return fmt.Errorf("%w: record key %q", ErrInvalidKey, key)
		}
	}
	return nil
}

// CheckDir is CheckKey for a folder, where "" names the top.
func CheckDir(dir string) error {
	if dir == "" {
		return nil
	}
	return CheckKey(dir)
}

// KeysUnder keeps the keys that sit below dir, byte for byte. "" keeps them all. A backend whose
// query can over-match, such as a LIKE that ignores case, filters its answer through it.
func KeysUnder(dir string, keys []string) []string {
	if dir == "" {
		return keys
	}
	out := keys[:0:0]
	for _, k := range keys {
		if strings.HasPrefix(k, dir+"/") {
			out = append(out, k)
		}
	}
	return out
}

// EntriesUnder turns the full keys below dir into what List returns: one entry per name directly
// under dir, a folder when more segments follow, ordered by name. It ignores keys not below dir.
// A backend that can only list keys, such as SQL or Datastore, answers List through it.
func EntriesUnder(dir string, keys []string) []Entry {
	prefix := ""
	if dir != "" {
		prefix = dir + "/"
	}
	var out []Entry
	seen := map[string]bool{}
	for _, k := range KeysUnder(dir, keys) {
		name, _, isDir := strings.Cut(strings.TrimPrefix(k, prefix), "/")
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, Entry{Name: name, Dir: isDir})
	}
	sortEntries(out)
	return out
}

// Join joins key segments with slashes. It does not check the result.
func Join(parts ...string) string { return strings.Join(parts, "/") }

// Sub returns the part of s under prefix, so a store's records sit under one folder and its keys
// leave the prefix off. An empty prefix returns s.
func Sub(s Store, prefix string) Store {
	if prefix == "" {
		return s
	}
	if inner, ok := s.(sub); ok {
		return sub{inner.s, inner.prefix + "/" + prefix}
	}
	return sub{s, prefix}
}

type sub struct {
	s      Store
	prefix string
}

func (p sub) key(k string) (string, error) {
	if err := CheckKey(k); err != nil {
		return "", err
	}
	return p.prefix + "/" + k, nil
}

func (p sub) dir(d string) (string, error) {
	if d == "" {
		return p.prefix, nil
	}
	return p.key(d)
}

func (p sub) Get(ctx context.Context, k string) ([]byte, error) {
	full, err := p.key(k)
	if err != nil {
		return nil, err
	}
	return p.s.Get(ctx, full)
}

func (p sub) Put(ctx context.Context, k string, body []byte) error {
	full, err := p.key(k)
	if err != nil {
		return err
	}
	return p.s.Put(ctx, full, body)
}

func (p sub) Create(ctx context.Context, k string, body []byte) error {
	full, err := p.key(k)
	if err != nil {
		return err
	}
	return p.s.Create(ctx, full, body)
}

func (p sub) Delete(ctx context.Context, k string) error {
	full, err := p.key(k)
	if err != nil {
		return err
	}
	return p.s.Delete(ctx, full)
}

func (p sub) List(ctx context.Context, d string) ([]Entry, error) {
	full, err := p.dir(d)
	if err != nil {
		return nil, err
	}
	return p.s.List(ctx, full)
}

func (p sub) Read(ctx context.Context, k string) ([]byte, Version, error) {
	full, err := p.key(k)
	if err != nil {
		return nil, "", err
	}
	return p.s.Read(ctx, full)
}

func (p sub) Replace(ctx context.Context, k string, body []byte, ver Version) error {
	full, err := p.key(k)
	if err != nil {
		return err
	}
	return p.s.Replace(ctx, full, body, ver)
}

func (p sub) DeleteDir(ctx context.Context, d string) error {
	full, err := p.dir(d)
	if err != nil {
		return err
	}
	return p.s.DeleteDir(ctx, full)
}
