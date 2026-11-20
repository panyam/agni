package projects

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/panyam/goapplib/storage/mounts"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// linkedWrite is a write of one netlist artifact read from the mount "repo" at version.
func linkedWrite(version, edif string) (service.ArtifactWrite, service.BlobSource) {
	u := uploadOf(map[string]string{"a.edn": edif})
	return service.ArtifactWrite{ID: "netlist", Files: u.files, AllowMissing: true,
		Source: &webapi.ArtifactSource{Mount: "repo", Path: "hw", Ref: "main", Follow: true, Version: version, ReadHash: "sha256:" + version}}, u.source
}

// readEntry reads the file a design's entry names, from the store's views.
func readEntry(t *testing.T, l layout, d *webapi.Design) string {
	t.Helper()
	u, err := mounts.ParseURI(d.GetEntryUri())
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(l.views(), filepath.FromSlash(u.Path)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// A revision holds the design's files at another source version in a view of its own, and leaves
// the design's record alone. It is found again by its id or a ref, listed newest first, kept by a
// sweep, and removed with its design (agni issue 1725).
func TestARevisionKeepsItsFilesBesideTheDesign(t *testing.T) {
	ctx := context.Background()
	ds, l := dirStore(t)
	t0 := time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)
	ds.now = func() time.Time { return t0 }
	if _, err := ds.CreateDesign(ctx, "bb", service.DesignRecord{}); err != nil {
		t.Fatal(err)
	}
	w, blobs := linkedWrite("c2", "(edif v2)")
	if _, err := ds.WriteArtifacts(ctx, "bb", []service.ArtifactWrite{w}, blobs); err != nil {
		t.Fatal(err)
	}

	ds.now = func() time.Time { return t0.Add(time.Hour) }
	w, blobs = linkedWrite("c1", "(edif v1)")
	rv, d, err := ds.PutRevision(ctx, "bb", "1.1.2", []service.ArtifactWrite{w}, blobs)
	if err != nil {
		t.Fatal(err)
	}
	if got := readEntry(t, l, d); got != "(edif v1)" {
		t.Errorf("the revision's entry reads %q; want the 1.1.2 files", got)
	}
	if d.GetName() != "designs/bb@"+rv.GetRevisionId() || !slices.Equal(rv.GetRefs(), []string{"1.1.2"}) || rv.GetSourceVersions()["netlist"] != "c1" || rv.GetCurrent() {
		t.Errorf("revision = %v, design %s; want designs/bb@<id>, ref 1.1.2, netlist at c1, not current", rv, d.GetName())
	}
	now, err := ds.Design(ctx, "bb")
	if err != nil {
		t.Fatal(err)
	}
	if got := readEntry(t, l, now); got != "(edif v2)" {
		t.Errorf("after the revision, the design reads %q; want its own files", got)
	}

	for _, rev := range []string{"1.1.2", rv.GetRevisionId()} {
		if got, _, err := ds.Revision(ctx, "bb", rev); err != nil || got.GetRevisionId() != rv.GetRevisionId() {
			t.Errorf("Revision(%q) = %v, %v; want %s", rev, got, err, rv.GetRevisionId())
		}
	}
	if _, _, err := ds.Revision(ctx, "bb", "1.1.9"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("Revision(1.1.9) = %v; want not found", err)
	}
	again, _, err := ds.PutRevision(ctx, "bb", "c1", []service.ArtifactWrite{w}, blobs)
	if err != nil || again.GetRevisionId() != rv.GetRevisionId() || !slices.Equal(again.GetRefs(), []string{"1.1.2", "c1"}) {
		t.Errorf("the same files under another ref = %v, %v; want one revision with both refs", again, err)
	}

	all, err := ds.Revisions(ctx, "bb")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].GetRevisionId() != rv.GetRevisionId() || all[0].GetCurrent() || !all[1].GetCurrent() || all[1].GetSourceVersions()["netlist"] != "c2" {
		t.Fatalf("revisions = %v; want the 1.1.2 read first, then the current one the link kept", all)
	}

	if _, _, err := ds.Sweep(nil, 0); err != nil {
		t.Fatal(err)
	}
	if _, d, err := ds.Revision(ctx, "bb", "1.1.2"); err != nil {
		t.Fatal(err)
	} else if got := readEntry(t, l, d); got != "(edif v1)" {
		t.Errorf("after a sweep the revision reads %q", got)
	}

	if ids, err := ds.ids(); err != nil || !slices.Equal(ids, []string{"bb"}) {
		t.Errorf("ids = %v, %v; want bb alone, since the revisions folder is not a design", ids, err)
	}
	if err := ds.DeleteDesign(ctx, "bb"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(l.records, revisionsDir, "bb")); !os.IsNotExist(err) {
		t.Errorf("after DeleteDesign, its revisions = %v; want them gone", err)
	}
	if views, _, err := ds.Sweep(nil, 0); err != nil || views != 2 {
		t.Errorf("a sweep after the delete removed %d views, %v; want the design's and the revision's", views, err)
	}
}
