package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/mounts"
)

// --store-dir adds a writable folder as the mount "store", and the server's project store reads what
// a DirStore writes there (agni issue 1693).
func TestStoreDirServesAsTheStoreMount(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "not-yet")
	ms, err := withStoreMount([]mounts.Mount{{Name: "tut", Root: t.TempDir()}}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 || ms[1].Name != storeMount || ms[1].Root != dir {
		t.Fatalf("mounts = %v, want tut then store at %s", ms, dir)
	}
	read := projects.NewFSStore(projectTrees(ms)...)
	ds, err := projects.NewDirStore(dir, read)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("title: B200\n")
	sum := sha256.Sum256(body)
	h := "sha256:" + hex.EncodeToString(sum[:])
	files := []*webapi.SnapshotFile{{Path: "project.yaml", Sha256: h, Size: int64(len(body))}}
	blobs := func(_ context.Context, got string) ([]byte, bool) { return body, got == h }
	if _, err := ds.CreateProject(context.Background(), "b200", files, blobs); err != nil {
		t.Fatal(err)
	}
	ps, err := read.Projects(context.Background())
	if err != nil || len(ps) != 1 || ps[0].GetName() != "projects/b200" || ps[0].GetTitle() != "B200" {
		t.Fatalf("projects served = %v, %v; want projects/b200 titled B200", ps, err)
	}
}

func TestStoreDirRefusesAMountNamedStore(t *testing.T) {
	_, err := withStoreMount([]mounts.Mount{{Name: storeMount, Root: t.TempDir()}}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "already declared") {
		t.Fatalf("withStoreMount over a mount named store = %v, want a refusal", err)
	}
}
