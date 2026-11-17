package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/mounts"
)

// --store-dir mounts its designs folder as "store", and the server's project store reads a design a
// DirStore writes there as a design of no project (agni issues 1693, 1694).
func TestStoreDirServesAsTheStoreMount(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "not-yet")
	ms, err := withStoreMount([]mounts.Mount{{Name: "tut", Root: t.TempDir()}}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 || ms[1].Name != storeMount || ms[1].Root != filepath.Join(dir, storeDesignsDir) {
		t.Fatalf("mounts = %v, want tut then store at %s", ms, filepath.Join(dir, storeDesignsDir))
	}
	ds, err := projects.NewDirStore(ms[1].Root, storeMount)
	if err != nil {
		t.Fatal(err)
	}
	var files []*webapi.SnapshotFile
	blobs := map[string][]byte{}
	for _, f := range []struct{ path, body string }{{"design.yaml", "title: Memory unit\nentry: mem.edn\n"}, {"mem.edn", "(edif mem)"}} {
		sum := sha256.Sum256([]byte(f.body))
		h := "sha256:" + hex.EncodeToString(sum[:])
		files = append(files, &webapi.SnapshotFile{Path: f.path, Sha256: h, Size: int64(len(f.body))})
		blobs[h] = []byte(f.body)
	}
	source := func(_ context.Context, h string) ([]byte, bool) { b, ok := blobs[h]; return b, ok }
	if _, err := ds.CreateDesign(context.Background(), "memory_unit", files, source); err != nil {
		t.Fatal(err)
	}
	read := projects.NewFSStore(projectTrees(ms)...)
	u, err := artifact.Parse("mount://store/memory_unit/mem.edn")
	if err != nil {
		t.Fatal(err)
	}
	d, p, err := read.ResolveDesign(context.Background(), u)
	if err != nil || d == nil || p != nil || d.GetTitle() != "Memory unit" {
		t.Fatalf("served design = %v, %v, %v; want Memory unit with no project", d, p, err)
	}
}

func TestStoreDirRefusesAMountNamedStore(t *testing.T) {
	_, err := withStoreMount([]mounts.Mount{{Name: storeMount, Root: t.TempDir()}}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "already declared") {
		t.Fatalf("withStoreMount over a mount named store = %v, want a refusal", err)
	}
}
