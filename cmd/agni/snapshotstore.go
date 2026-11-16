package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
	"google.golang.org/protobuf/encoding/protojson"
)

// osSnapshotStore is the filesystem-backed service.SnapshotStore, under `agni serve --state-dir`
// (agni issue 1639). It writes one small protojson file per snapshot, the definition a client sent,
// so a server that restarted ingests the snapshot again when a question names it. The directory is a
// WRITABLE volume, apart from the read-only design mounts, like --review-store's.
type osSnapshotStore struct{ dir string }

// newOSSnapshotStore returns a store over dir, creating it when absent. A path that exists and is
// not a directory is an error at startup.
func newOSSnapshotStore(dir string) (*osSnapshotStore, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("--state-dir: %w", err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("--state-dir: %s is not a directory", dir)
	}
	return &osSnapshotStore{dir: dir}, nil
}

// path is where name's definition lives. A name is "snapshots/{id}", and an id that is empty, starts
// with a dot or holds a separator names nothing, so no name reaches outside the directory.
func (s *osSnapshotStore) path(name string) (string, bool) {
	id, ok := strings.CutPrefix(name, "snapshots/")
	if !ok || id == "" || strings.HasPrefix(id, ".") || strings.ContainsAny(id, `/\:`) {
		return "", false
	}
	return filepath.Join(s.dir, id+".json"), true
}

func (s *osSnapshotStore) Put(_ context.Context, name string, def *webapi.Snapshot) error {
	p, ok := s.path(name)
	if !ok {
		return fmt.Errorf("%w: %q is no snapshot name", service.ErrInvalidArgument, name)
	}
	b, err := protojson.Marshal(def)
	if err != nil {
		return err
	}
	// Written beside its final name and renamed over it, so a crash leaves the old definition or the
	// new one, never half of one.
	tmp, err := os.CreateTemp(s.dir, ".put-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), p)
}

func (s *osSnapshotStore) Get(_ context.Context, name string) (*webapi.Snapshot, error) {
	p, ok := s.path(name)
	if !ok {
		return nil, service.ErrNotFound
	}
	b, err := os.ReadFile(p) // #nosec G304 -- a path under the store's own directory, checked by path
	if errors.Is(err, os.ErrNotExist) {
		return nil, service.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	def := &webapi.Snapshot{}
	if err := protojson.Unmarshal(b, def); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	return def, nil
}

func (s *osSnapshotStore) Delete(_ context.Context, name string) error {
	p, ok := s.path(name)
	if !ok {
		return nil
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Persistent is true: the definitions outlive the process.
func (s *osSnapshotStore) Persistent() bool { return true }
