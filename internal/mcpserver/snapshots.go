package mcpserver

import (
	"context"
	"errors"
	"sync"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// ingester is the half of the SnapshotService the designs cache calls.
type ingester interface {
	Ingest(context.Context, *webapi.IngestRequest) (*webapi.Snapshot, error)
	DeleteSnapshot(context.Context, *webapi.DeleteSnapshotRequest) (*emptypb.Empty, error)
}

// designs keeps one snapshot per design an agent has named, so a conversation of questions about one
// design reads it once (agni issue 1061). A snapshot the host lost, or whose files changed since it
// was read, is ingested again on the next question rather than failing it.
//
// Each design has a lock of its own, held while it is read, so a question about one design waits for
// that design alone and never for another being read (agni issue 1121).
type designs struct {
	svc  ingester
	mu   sync.Mutex
	held map[string]*held
}

// held is one design's snapshot, nil until its first read succeeds.
type held struct {
	mu   sync.Mutex
	snap *webapi.Snapshot
}

func newDesigns(svc ingester) *designs {
	return &designs{svc: svc, held: map[string]*held{}}
}

func (d *designs) entry(u artifact.URI) *held {
	d.mu.Lock()
	defer d.mu.Unlock()
	h := d.held[u.String()]
	if h == nil {
		h = &held{}
		d.held[u.String()] = h
	}
	return h
}

// snapshot is the snapshot held for u, ingesting the design the first time it is named.
func (d *designs) snapshot(ctx context.Context, u artifact.URI) (*webapi.Snapshot, error) {
	h := d.entry(u)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.snap != nil {
		return h.snap, nil
	}
	s, err := d.svc.Ingest(ctx, &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: u.String()}})
	if err != nil {
		return nil, err
	}
	h.snap = s
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
	h := d.entry(u)
	h.mu.Lock()
	if h.snap == s {
		h.snap = nil
	}
	h.mu.Unlock()
	_, _ = d.svc.DeleteSnapshot(context.WithoutCancel(ctx), &webapi.DeleteSnapshotRequest{Name: s.GetName()})
}
