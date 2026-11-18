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
	"github.com/panyam/agni/service"
)

// --uploads serves its design views as the mount "uploads", and the server's project store reads a
// design a DirStore writes there as a design of no project (agni issues 1693, 1694, 1739).
func TestUploadsServeAsTheUploadsMount(t *testing.T) {
	uploads := filepath.Join(t.TempDir(), "not-yet")
	ms, err := withUploadsMount([]mounts.Mount{{Name: "tut", Root: t.TempDir()}}, uploads)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 || ms[1].Name != uploadsMount || ms[1].Root != projects.ViewsRoot(uploads) {
		t.Fatalf("mounts = %v, want tut then uploads at %s", ms, projects.ViewsRoot(uploads))
	}
	ds, err := projects.NewDirStore(t.TempDir(), uploads, uploadsMount)
	if err != nil {
		t.Fatal(err)
	}
	var files []*webapi.SnapshotFile
	blobs := map[string][]byte{}
	for _, f := range []struct{ path, body string }{{"mem.edn", "(edif mem)"}} {
		sum := sha256.Sum256([]byte(f.body))
		h := "sha256:" + hex.EncodeToString(sum[:])
		files = append(files, &webapi.SnapshotFile{Path: f.path, Sha256: h, Size: int64(len(f.body))})
		blobs[h] = []byte(f.body)
	}
	source := func(_ context.Context, h string) ([]byte, bool) { b, ok := blobs[h]; return b, ok }
	if _, err := ds.CreateDesign(context.Background(), "memory_unit", service.DesignRecord{Title: "Memory unit"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ds.WriteArtifacts(context.Background(), "memory_unit", []service.ArtifactWrite{{ID: "netlist", Files: files, AllowMissing: true}}, source); err != nil {
		t.Fatal(err)
	}
	stored, err := ds.Design(context.Background(), "memory_unit")
	if err != nil || !inUploads("memory_unit", "netlist/mem.edn").MatchString(stored.GetEntryUri()) {
		t.Fatalf("stored design = %v, %v; want its netlist in the uploads mount", stored, err)
	}
	read := projects.NewFSStore(projectTrees(ms)...)
	u, err := artifact.Parse(stored.GetEntryUri())
	if err != nil {
		t.Fatal(err)
	}
	d, p, err := read.ResolveDesign(context.Background(), u)
	if err != nil || d == nil || p != nil || d.GetTitle() != "Memory unit" {
		t.Fatalf("served design = %v, %v, %v; want Memory unit with no project", d, p, err)
	}
}

func TestUploadsRefuseAMountNamedUploads(t *testing.T) {
	_, err := withUploadsMount([]mounts.Mount{{Name: uploadsMount, Root: t.TempDir()}}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "already declared") {
		t.Fatalf("withUploadsMount over a mount named uploads = %v, want a refusal", err)
	}
}

func TestStoreAndUploadsTakeFileURLs(t *testing.T) {
	for _, tc := range []struct {
		store, uploads string
		wantStore      string
		wantUploads    string
		wantErr        string
	}{
		{store: "", uploads: ""},
		{store: "file:///srv/agni/meta", wantStore: "/srv/agni/meta"},
		{store: "file:///srv/agni/meta", uploads: "file:///srv/agni/files", wantStore: "/srv/agni/meta", wantUploads: "/srv/agni/files"},
		{store: "file://localhost/srv/agni/meta", wantStore: "/srv/agni/meta"},
		{store: "/srv/agni/meta", wantErr: "write it as file:///srv/agni/meta"},
		{store: "sqlite:///srv/agni.db", wantErr: "agni issue 1733"},
		{store: "postgres://db/agni", wantErr: "this build knows file://, sqlite://"},
		{store: "file://server/srv/agni", wantErr: "names the host"},
		{store: "file://relative", wantErr: "names the host"},
		{store: "file:///srv/agni/meta", uploads: "s3://bucket/agni", wantErr: "agni issue 1730"},
		{uploads: "file:///srv/agni/files", wantErr: "needs --store"},
	} {
		store, uploads, err := storeFolders(tc.store, tc.uploads)
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("--store %q --uploads %q = %v, want an error naming %q", tc.store, tc.uploads, err, tc.wantErr)
			}
			continue
		}
		if err != nil || store != filepath.FromSlash(tc.wantStore) || uploads != filepath.FromSlash(tc.wantUploads) {
			t.Errorf("--store %q --uploads %q = %q, %q, %v; want %q, %q", tc.store, tc.uploads, store, uploads, err, tc.wantStore, tc.wantUploads)
		}
	}
}
