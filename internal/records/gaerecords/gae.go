// Package gaerecords keeps a server's records in Google Cloud Datastore, or Firestore in Datastore
// mode, for `--store datastore://` (agni issue 1733). It is apart from package records so that a
// program reading only folders never links the Datastore SDK.
package gaerecords

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/datastore"

	"github.com/panyam/agni/internal/records"
)

// The entity kinds. They are the schema, so renaming a Go type doesn't change them.
const (
	recordKind = "AgniRecord"
	partKind   = "AgniRecordPart"
)

// partSize is the most body bytes one entity holds. Datastore caps an entity near 1 MiB with its
// key and properties, and a stored report runs to 15 MB, so a body past this is split into parts.
const partSize = 960 << 10

// putBatch is how many parts go in one PutMulti or GetMulti. Datastore takes a 10 MiB commit, but a gRPC server
// receives 4 MiB per message by default, and the emulator cancels a larger one. Three parts stay
// under both.
const putBatch = 3

// recordEntity is the head of one record, keyed by the record's key. A small body sits in Body. A
// large one sits in Parts entities named for Version, which the head names only once they are all
// written.
type recordEntity struct {
	Body    []byte    `datastore:"body,noindex"`
	Version string    `datastore:"version,noindex"`
	Parts   int       `datastore:"parts,noindex"`
	Updated time.Time `datastore:"updated,noindex"`
	// Rev counts the record's writes. Replace writes only when it still holds the count Read saw.
	Rev int64 `datastore:"rev,noindex"`
}

// partEntity is one slice of a large body. Its key's parent is the record's head key.
type partEntity struct {
	Body []byte `datastore:"body,noindex"`
}

// GAE is a Store over Datastore, keeping each record as an entity of kind AgniRecord in one
// namespace. The record's key is the entity's key name, and Datastore orders key names byte by
// byte, so List finds a folder's records with a key range whatever the locale.
//
// A body larger than partSize is written as parts first, then the head is switched to them in a
// transaction, so a reader sees the old record or the new one. A writer that dies between the two
// leaves parts no head names. Nothing reads them, and Sweep deletes them once they are older than a
// write could take.
type GAE struct {
	client    *datastore.Client
	namespace string
	now       func() time.Time
}

// New returns a Store over client, keeping records in namespace. "" is the default namespace.
func New(client *datastore.Client, namespace string) *GAE {
	return &GAE{client: client, namespace: namespace, now: time.Now}
}

var _ records.Store = (*GAE)(nil)

func (s *GAE) key(k string) *datastore.Key {
	key := datastore.NameKey(recordKind, k, nil)
	key.Namespace = s.namespace
	return key
}

func (s *GAE) partKeys(head *datastore.Key, version string, n int) []*datastore.Key {
	out := make([]*datastore.Key, n)
	for i := range out {
		out[i] = datastore.NameKey(partKind, fmt.Sprintf("%s.%05d", version, i), head)
		out[i].Namespace = s.namespace
	}
	return out
}

// Get reads the head, then its parts when the body is split.
func (s *GAE) Get(ctx context.Context, k string) ([]byte, error) {
	b, _, err := s.Read(ctx, k)
	return b, err
}

// Read reads the head, then its parts when the body is split, and returns the head's rev.
func (s *GAE) Read(ctx context.Context, k string) ([]byte, records.Version, error) {
	if err := records.CheckKey(k); err != nil {
		return nil, "", err
	}
	var head recordEntity
	if err := s.client.Get(ctx, s.key(k), &head); errors.Is(err, datastore.ErrNoSuchEntity) {
		return nil, "", fmt.Errorf("record %s: %w", k, fs.ErrNotExist)
	} else if err != nil {
		return nil, "", err
	}
	ver := records.Version(strconv.FormatInt(head.Rev, 10))
	if head.Parts == 0 {
		return head.Body, ver, nil
	}
	// The parts come back putBatch at a time, since a gRPC client also receives 4 MiB per message.
	keys := s.partKeys(s.key(k), head.Version, head.Parts)
	var out []byte
	for i := 0; i < len(keys); i += putBatch {
		parts := make([]partEntity, min(putBatch, len(keys)-i))
		if err := s.client.GetMulti(ctx, keys[i:i+len(parts)], parts); err != nil {
			return nil, "", fmt.Errorf("record %s: %w", k, err)
		}
		for _, p := range parts {
			out = append(out, p.Body...)
		}
	}
	return out, ver, nil
}

// Put writes key, replacing any record it had.
func (s *GAE) Put(ctx context.Context, k string, body []byte) error {
	return s.write(ctx, k, body, func(*recordEntity) error { return nil })
}

// Create writes key in a transaction that first checks no head exists.
func (s *GAE) Create(ctx context.Context, k string, body []byte) error {
	return s.write(ctx, k, body, func(old *recordEntity) error {
		if old != nil {
			return fmt.Errorf("record %s: %w", k, fs.ErrExist)
		}
		return nil
	})
}

// Replace writes key in a transaction that first checks the head's rev is still ver.
func (s *GAE) Replace(ctx context.Context, k string, body []byte, ver records.Version) error {
	return s.write(ctx, k, body, func(old *recordEntity) error {
		switch {
		case old == nil:
			return fmt.Errorf("record %s: %w", k, fs.ErrNotExist)
		case strconv.FormatInt(old.Rev, 10) != string(ver):
			return fmt.Errorf("record %s: %w", k, records.ErrConflict)
		}
		return nil
	})
}

// write puts a large body's parts, then the head in a transaction, then deletes the parts of the
// version it replaced. check sees the head the transaction read, nil for none, and an error from it
// fails the write and deletes the new parts.
func (s *GAE) write(ctx context.Context, k string, body []byte, check func(old *recordEntity) error) error {
	if err := records.CheckKey(k); err != nil {
		return err
	}
	headKey := s.key(k)
	head := recordEntity{Updated: s.now().UTC()}
	var newParts []*datastore.Key
	if len(body) <= partSize {
		head.Body = body
	} else {
		v, err := s.version()
		if err != nil {
			return err
		}
		n := (len(body) + partSize - 1) / partSize
		head.Version, head.Parts = v, n
		newParts = s.partKeys(headKey, v, n)
		for i := 0; i < n; i += putBatch {
			j := min(i+putBatch, n)
			ents := make([]*partEntity, 0, j-i)
			for p := i; p < j; p++ {
				ents = append(ents, &partEntity{Body: body[p*partSize : min((p+1)*partSize, len(body))]})
			}
			if _, err := s.client.PutMulti(ctx, newParts[i:j], ents); err != nil {
				s.deleteParts(ctx, newParts)
				return err
			}
		}
	}
	var old recordEntity
	var hadOld bool
	_, err := s.client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		old, hadOld = recordEntity{}, false
		switch err := tx.Get(headKey, &old); {
		case err == nil:
			hadOld = true
		case !errors.Is(err, datastore.ErrNoSuchEntity):
			return err
		}
		var seen *recordEntity
		if hadOld {
			seen = &old
		}
		if err := check(seen); err != nil {
			return err
		}
		next := head
		next.Rev = old.Rev + 1
		_, err := tx.Put(headKey, &next)
		return err
	}, datastore.MaxAttempts(16))
	if err != nil {
		s.deleteParts(ctx, newParts)
		return err
	}
	if hadOld && old.Parts > 0 {
		s.deleteParts(ctx, s.partKeys(headKey, old.Version, old.Parts))
	}
	return nil
}

// deleteParts removes parts nothing names any more. A failure leaves them behind, unread.
func (s *GAE) deleteParts(ctx context.Context, keys []*datastore.Key) {
	for i := 0; i < len(keys); i += 500 {
		_ = s.client.DeleteMulti(ctx, keys[i:min(i+500, len(keys))])
	}
}

// Delete removes the head in a transaction, then its parts.
func (s *GAE) Delete(ctx context.Context, k string) error {
	if err := records.CheckKey(k); err != nil {
		return err
	}
	headKey := s.key(k)
	var old recordEntity
	_, err := s.client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		if err := tx.Get(headKey, &old); errors.Is(err, datastore.ErrNoSuchEntity) {
			return fmt.Errorf("record %s: %w", k, fs.ErrNotExist)
		} else if err != nil {
			return err
		}
		return tx.Delete(headKey)
	}, datastore.MaxAttempts(16))
	if err != nil {
		return err
	}
	if old.Parts > 0 {
		s.deleteParts(ctx, s.partKeys(headKey, old.Version, old.Parts))
	}
	return nil
}

// keysUnder lists the record keys below dir with a keys-only range query. Every key under "a/b"
// sorts from "a/b/" up to "a/b0", since '0' follows '/' and Datastore compares names as bytes.
func (s *GAE) keysUnder(ctx context.Context, dir string) ([]string, error) {
	q := datastore.NewQuery(recordKind).Namespace(s.namespace).KeysOnly()
	if dir != "" {
		q = q.FilterField("__key__", ">=", s.key(dir+"/")).FilterField("__key__", "<", s.key(dir+"0"))
	}
	keys, err := s.client.GetAll(ctx, q, nil)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = k.Name
	}
	return records.KeysUnder(dir, out), nil
}

// List reads the keys below dir and keeps the segment after dir of each.
func (s *GAE) List(ctx context.Context, dir string) ([]records.Entry, error) {
	if err := records.CheckDir(dir); err != nil {
		return nil, err
	}
	keys, err := s.keysUnder(ctx, dir)
	if err != nil {
		return nil, err
	}
	return records.EntriesUnder(dir, keys), nil
}

// DeleteDir deletes each record below dir. A record another writer adds meanwhile may stay.
func (s *GAE) DeleteDir(ctx context.Context, dir string) error {
	if err := records.CheckDir(dir); err != nil {
		return err
	}
	keys, err := s.keysUnder(ctx, dir)
	if err != nil {
		return err
	}
	for _, k := range keys {
		if err := s.Delete(ctx, k); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

// version names one write's parts, so a new body's parts never meet an old one's. It starts with the
// write's time in hex nanoseconds, so Sweep can tell a part's age from its key alone.
func (s *GAE) version() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return strconv.FormatInt(s.now().UnixNano(), 16) + "-" + hex.EncodeToString(b[:]), nil
}

// writtenAt reads the time a part's version starts with, and false for a name it can't read.
func writtenAt(partName string) (time.Time, bool) {
	ns, _, ok := strings.Cut(partName, "-")
	if !ok {
		return time.Time{}, false
	}
	n, err := strconv.ParseInt(ns, 16, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(0, n), true
}

// Sweep deletes the part entities no head names that were written more than grace ago, and returns
// how many (agni issue 1823). A writer that died between writing a large body's parts and switching
// the head to them leaves such parts. grace spares a write still in flight, and a part whose name
// it can't read is left alone.
func (s *GAE) Sweep(ctx context.Context, grace time.Duration) (int, error) {
	keys, err := s.client.GetAll(ctx, datastore.NewQuery(partKind).Namespace(s.namespace).KeysOnly(), nil)
	if err != nil {
		return 0, err
	}
	cutoff := s.now().Add(-grace)
	heads := map[string]*recordEntity{}
	var gone []*datastore.Key
	for _, k := range keys {
		at, ok := writtenAt(k.Name)
		if !ok || at.After(cutoff) || k.Parent == nil {
			continue
		}
		head, seen := heads[k.Parent.Name]
		if !seen {
			var h recordEntity
			switch err := s.client.Get(ctx, k.Parent, &h); {
			case err == nil:
				head = &h
			case !errors.Is(err, datastore.ErrNoSuchEntity):
				return 0, err
			}
			heads[k.Parent.Name] = head
		}
		v, _, _ := strings.Cut(k.Name, ".")
		if head == nil || head.Parts == 0 || head.Version != v {
			gone = append(gone, k)
		}
	}
	s.deleteParts(ctx, gone)
	return len(gone), nil
}
