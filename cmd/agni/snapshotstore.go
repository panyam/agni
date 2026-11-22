package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/records"
	"github.com/panyam/agni/service"
	"google.golang.org/protobuf/encoding/protojson"
)

// osSnapshotStore is the service.SnapshotStore under `agni serve --store` (agni issues 1639, 1733).
// It keeps one small protojson record per snapshot, the definition a client sent, so a server that
// restarted ingests the snapshot again when a question names it. The records sit under the store's
// snapshots folder, in files or in sqlite.
type osSnapshotStore struct{ recs records.Store }

// newOSSnapshotStore returns a store over the folder dir, creating it when absent. A path that
// exists and is not a directory is an error at startup.
func newOSSnapshotStore(dir string) (*osSnapshotStore, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("--store: %w", err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("--store: %s is not a directory", dir)
	}
	d, err := records.NewDir(dir)
	if err != nil {
		return nil, fmt.Errorf("--store: %w", err)
	}
	return &osSnapshotStore{recs: d}, nil
}

// key is where name's definition lives. A name is "snapshots/{id}", and an id that is empty, starts
// with a dot or holds a separator names nothing, so no name reaches outside the folder.
func (s *osSnapshotStore) key(name string) (string, bool) {
	id, ok := strings.CutPrefix(name, "snapshots/")
	if !ok || id == "" || strings.HasPrefix(id, ".") || strings.ContainsAny(id, `/\:`) {
		return "", false
	}
	return id + ".json", true
}

func (s *osSnapshotStore) Put(ctx context.Context, name string, def *webapi.Snapshot) error {
	k, ok := s.key(name)
	if !ok {
		return fmt.Errorf("%w: %q is no snapshot name", service.ErrInvalidArgument, name)
	}
	b, err := protojson.Marshal(def)
	if err != nil {
		return err
	}
	return s.recs.Put(ctx, k, b)
}

func (s *osSnapshotStore) Get(ctx context.Context, name string) (*webapi.Snapshot, error) {
	k, ok := s.key(name)
	if !ok {
		return nil, service.ErrNotFound
	}
	b, err := s.recs.Get(ctx, k)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, service.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	def := &webapi.Snapshot{}
	if err := protojson.Unmarshal(b, def); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return def, nil
}

func (s *osSnapshotStore) Delete(ctx context.Context, name string) error {
	k, ok := s.key(name)
	if !ok {
		return nil
	}
	if err := s.recs.Delete(ctx, k); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Hashes returns the hash of every file a stored definition sent, so the uploads sweep keeps them.
// A store holding no definitions returns none.
func (s *osSnapshotStore) Hashes(ctx context.Context) (map[string]bool, error) {
	out := map[string]bool{}
	entries, err := s.recs.List(ctx, "")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.Dir || !strings.HasSuffix(e.Name, ".json") {
			continue
		}
		b, err := s.recs.Get(ctx, e.Name)
		if err != nil {
			return nil, err
		}
		def := &webapi.Snapshot{}
		if err := protojson.Unmarshal(b, def); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name, err)
		}
		for _, f := range def.GetFiles() {
			out[f.GetSha256()] = true
		}
	}
	return out, nil
}

// Persistent is true: the definitions outlive the process.
func (s *osSnapshotStore) Persistent() bool { return true }
