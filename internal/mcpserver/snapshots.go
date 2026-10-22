package mcpserver

import (
	"context"
	"errors"
	"sync"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// designs keeps one snapshot per design an agent has named, so a conversation of questions about one
// design reads it once (agni issue 1061). A snapshot the host lost, or whose files changed since it
// was read, is ingested again on the next question rather than failing it.
type designs struct {
	svc  *service.SnapshotService
	mu   sync.Mutex
	held map[string]*webapi.Snapshot
}

func newDesigns(svc *service.SnapshotService) *designs {
	return &designs{svc: svc, held: map[string]*webapi.Snapshot{}}
}

// snapshot is the snapshot held for u, ingesting the design the first time it is named.
func (d *designs) snapshot(ctx context.Context, u artifact.URI) (*webapi.Snapshot, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if s, ok := d.held[u.String()]; ok {
		return s, nil
	}
	s, err := d.svc.Ingest(ctx, &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: u.String()}})
	if err != nil {
		return nil, err
	}
	d.held[u.String()] = s
	return s, nil
}

// ask runs fn over u's snapshot, and once more over a fresh one when fn reports the snapshot lost or
// stale, deleting the one it replaces.
func (d *designs) ask(ctx context.Context, u artifact.URI, fn func(*webapi.Snapshot) error) error {
	s, err := d.snapshot(ctx, u)
	if err != nil {
		return err
	}
	err = fn(s)
	var lost *service.SnapshotError
	if !errors.As(err, &lost) {
		return err
	}
	d.forget(ctx, u, s)
	if s, err = d.snapshot(ctx, u); err != nil {
		return err
	}
	return fn(s)
}

// forget drops s as u's snapshot, if it still is, and deletes it on the host.
func (d *designs) forget(ctx context.Context, u artifact.URI, s *webapi.Snapshot) {
	d.mu.Lock()
	if d.held[u.String()] == s {
		delete(d.held, u.String())
	}
	d.mu.Unlock()
	_, _ = d.svc.DeleteSnapshot(context.WithoutCancel(ctx), &webapi.DeleteSnapshotRequest{Name: s.GetName()})
}
