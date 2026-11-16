package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
	"google.golang.org/protobuf/proto"
)

// The state directory's store keeps a definition across instances, as across a restart, and forgets
// one on delete (agni issue 1639).
func TestOSSnapshotStoreKeepsADefinitionAcrossInstances(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "snapshots")
	ctx := context.Background()
	st, err := newOSSnapshotStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	def := &webapi.Snapshot{Uri: "mount://tut/designs/gateway", AsNamed: true, Refresh: true}
	if err := st.Put(ctx, "snapshots/abc", def); err != nil {
		t.Fatal(err)
	}
	again, err := newOSSnapshotStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := again.Get(ctx, "snapshots/abc")
	if err != nil || !proto.Equal(got, def) {
		t.Fatalf("a second store over the directory read %v (%v), want %v", got, err, def)
	}
	if !again.Persistent() {
		t.Error("the state directory's store says it ends with the process")
	}
	if err := again.Delete(ctx, "snapshots/abc"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Get(ctx, "snapshots/abc"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("a deleted definition answered %v, want not found", err)
	}
}

// A name that is no snapshot's, or would reach outside the directory, names nothing.
func TestOSSnapshotStoreRefusesNamesOutsideItsDirectory(t *testing.T) {
	st, err := newOSSnapshotStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, name := range []string{"snapshots/../x", "snapshots/a/b", "snapshots/", "snapshots/.hidden", "reviews/x", `snapshots/a\b`} {
		if err := st.Put(ctx, name, &webapi.Snapshot{}); !errors.Is(err, service.ErrInvalidArgument) {
			t.Errorf("Put(%q) = %v, want invalid_argument", name, err)
		}
		if _, err := st.Get(ctx, name); !errors.Is(err, service.ErrNotFound) {
			t.Errorf("Get(%q) = %v, want not found", name, err)
		}
	}
}
