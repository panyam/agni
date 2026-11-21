package projects

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

// pinnedWrite is a write of artifact id, sch a schematic or brd a board, read from the mount "repo"
// at ref, pinned there.
func pinnedWrite(id, ref, edif string) (service.ArtifactWrite, service.BlobSource) {
	ext := map[string]string{"sch": ".kicad_sch", "brd": ".kicad_pcb"}[id]
	u := uploadOf(map[string]string{"bb" + ext: edif})
	return service.ArtifactWrite{ID: id, Files: u.files, AllowMissing: true,
		Source: &webapi.ArtifactSource{Mount: "repo", Path: id, Ref: ref, Version: "v-" + ref, ReadHash: "sha256:" + id + ref}}, u.source
}

// Linking a design's artifacts one at a time keeps a revision after each link. A revision is named
// by the ref every artifact is pinned at, the name moves to the last step that holds it, and the
// unnamed steps go once the design moves on, unless something pins them (agni issue 1774).
func TestUnnamedRevisionsGoOnceTheDesignMovesOn(t *testing.T) {
	ctx := context.Background()
	ds, _ := dirStore(t)
	t0 := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)
	step := 0
	ds.now = func() time.Time { return t0.Add(time.Duration(step) * time.Minute) }
	var pinned string
	ds.WithRevisionPins(func(context.Context, string) (map[string]bool, error) {
		return map[string]bool{pinned: pinned != ""}, nil
	})
	if _, err := ds.CreateDesign(ctx, "bb", service.DesignRecord{}); err != nil {
		t.Fatal(err)
	}
	link := func(id, ref string) {
		t.Helper()
		step++
		w, blobs := pinnedWrite(id, ref, "(kicad "+id+" "+ref+")")
		if _, err := ds.WriteArtifacts(ctx, "bb", []service.ArtifactWrite{w}, blobs); err != nil {
			t.Fatal(err)
		}
	}
	kept := func() []string {
		t.Helper()
		all, err := ds.Revisions(ctx, "bb")
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, r := range all {
			name := r.GetRevisionId()
			if len(r.GetRefs()) > 0 {
				name = strings.Join(r.GetRefs(), ",")
			}
			if r.GetCurrent() {
				name += "*"
			}
			out = append(out, name)
		}
		return out
	}

	link("sch", "1.1.2")
	link("brd", "1.1.2")
	if got := kept(); !slices.Equal(got, []string{"1.1.2*"}) {
		t.Errorf("after linking both at 1.1.2, revisions = %v; want the step with both, named 1.1.2 and current", got)
	}
	link("sch", "1.1.3")
	half, err := ds.Revisions(ctx, "bb")
	if err != nil {
		t.Fatal(err)
	}
	if got := kept(); len(got) != 2 || got[1] != "1.1.2" || !strings.HasSuffix(got[0], "*") || strings.Contains(got[0], ".") {
		t.Errorf("halfway to 1.1.3, revisions = %v; want the unnamed current step, then 1.1.2", got)
	}
	pinned = half[0].GetRevisionId()
	link("brd", "1.1.3")
	if got := kept(); !slices.Equal(got, []string{"1.1.3*", pinned, "1.1.2"}) {
		t.Errorf("at 1.1.3 with the halfway step pinned, revisions = %v; want 1.1.3, the pinned step, then 1.1.2", got)
	}
	pinned = ""
	link("sch", "1.1.2")
	link("sch", "1.1.3")
	if got := kept(); !slices.Equal(got, []string{"1.1.3*", "1.1.2"}) {
		t.Errorf("after the pin went, revisions = %v; want 1.1.3 and 1.1.2 alone", got)
	}
	arts, err := ds.RevisionArtifacts(ctx, "bb", "1.1.2")
	if err != nil {
		t.Fatal(err)
	}
	var versions []string
	for _, a := range arts {
		versions = append(versions, a.GetName()+"@"+a.GetSource().GetVersion())
	}
	slices.Sort(versions)
	if !slices.Equal(versions, []string{"designs/bb/artifacts/brd@v-1.1.2", "designs/bb/artifacts/sch@v-1.1.2"}) {
		t.Errorf("RevisionArtifacts(1.1.2) = %v; want both artifacts at 1.1.2", versions)
	}
}
