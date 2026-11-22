package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/authgate"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/internal/records/recordstest"
	"github.com/panyam/agni/internal/storeurl"
	"github.com/panyam/agni/mountset"
	"github.com/panyam/agni/service"
	"github.com/panyam/goapplib/storage/mounts"
)

// --uploads serves its design views as the mount "uploads", and the server's project store reads a
// design a DirStore writes there as a design of no project (agni issues 1693, 1694, 1739).
func TestUploadsServeAsTheUploadsMount(t *testing.T) {
	uploads := filepath.Join(t.TempDir(), "not-yet")
	ms, err := withUploadsMount([]mounts.Mount{mountset.Folder("tut", t.TempDir())}, uploads)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 || ms[1].Name != uploadsMount || ms[1].Source != projects.ViewsRoot(uploads) {
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
	u, err := mounts.ParseURI(stored.GetEntryUri())
	if err != nil {
		t.Fatal(err)
	}
	d, p, err := read.ResolveDesign(context.Background(), u)
	if err != nil || d == nil || p != nil || d.GetTitle() != "Memory unit" {
		t.Fatalf("served design = %v, %v, %v; want Memory unit with no project", d, p, err)
	}
}

func TestUploadsRefuseAMountNamedUploads(t *testing.T) {
	_, err := withUploadsMount([]mounts.Mount{mountset.Folder(uploadsMount, t.TempDir())}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "already declared") {
		t.Fatalf("withUploadsMount over a mount named uploads = %v, want a refusal", err)
	}
}

// testStoreURL is the --store URL of a test server's store over the folder dir, in the backend
// AGNI_TEST_RECORDS names (agni issue 1733).
func testStoreURL(t testing.TB, dir string) string {
	t.Helper()
	return recordstest.StoreURL(t, dir)
}

// testStore opens testStoreURL(t, dir). Each call opens a new handle on the same records, as a
// restarted server does, and the test closes it when it ends.
func testStore(t testing.TB, dir string) *storeurl.Store {
	t.Helper()
	st, _, err := storeurl.Open(testStoreURL(t, dir), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// testTokens is the token store of the --store testStore opens over dir.
func testTokens(t testing.TB, dir string) authgate.Keys {
	t.Helper()
	k, err := testStore(t, dir).Tokens()
	if err != nil {
		t.Fatal(err)
	}
	return k
}
