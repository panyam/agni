package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

func TestTheTourKeepsTwoRevisionsAndRefusesTheMountedDesign(t *testing.T) {
	ctx := context.Background()
	tr, err := newTour(t.TempDir(), mounted)
	if err != nil {
		t.Fatal(err)
	}
	if _, asked, err := tr.keep(ctx, "mixer-rev-a", "Mixer rev A", "../common/designs/rev-a.edn"); err != nil || asked != 1 {
		t.Fatalf("keep rev A = asked %d, %v; want the netlist's contents asked for", asked, err)
	}
	if _, asked, err := tr.keep(ctx, "mixer-rev-b", "Mixer rev B", "../common/designs/rev-b.edn"); err != nil || asked != 1 {
		t.Fatalf("keep rev B = asked %d, %v; want its netlist's contents asked for", asked, err)
	}
	before, after, err := tr.keepPack(ctx)
	if err != nil || before.GetHash() == "" || before.GetHash() == after.GetHash() {
		t.Errorf("keepPack = %v, %v, %v; want a hash that moves when the rail pattern changes", before, after, err)
	}
	kept, read, err := tr.report(ctx)
	if err != nil || kept.GetDesignChanged() || !read.GetDesignChanged() {
		t.Errorf("report = %v, then %v, %v; want it current when kept and run on earlier files after the netlist changed", kept, read, err)
	}
	if _, err := tr.projects.CreateProject(ctx, &webapi.CreateProjectRequest{ProjectId: "lab", Project: &webapi.Project{Designs: []string{"designs/mixer", "designs/mixer-rev-a", "designs/mixer-rev-b"}}}); err != nil {
		t.Fatal(err)
	}
	list, err := tr.designs.ListDesigns(ctx, &webapi.ListDesignsRequest{Filter: `project = "projects/lab"`})
	if err != nil || len(list.GetDesigns()) != 3 {
		t.Errorf("projects/lab = %v, %v; want three designs", list.GetDesigns(), err)
	}
	if err := tr.designs.DeleteDesign(ctx, &webapi.DeleteDesignRequest{Name: "designs/mixer"}); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("DeleteDesign of the mounted mixer = %v, want ErrFailedPrecondition", err)
	}
	if err := tr.projects.DeleteProject(ctx, &webapi.DeleteProjectRequest{Name: "projects/lab"}); err != nil {
		t.Fatal(err)
	}
	if all, err := tr.designs.ListDesigns(ctx, &webapi.ListDesignsRequest{}); err != nil || len(all.GetDesigns()) != 3 {
		t.Errorf("designs after deleting the project = %v, %v; want all three", all.GetDesigns(), err)
	}
}

// A poll reads a followed folder's change into the design, and only notes the next one once the
// design is set to mark (agni issue 1726).
func TestTheTourFollowsAFolder(t *testing.T) {
	tr, err := newTour(t.TempDir(), mounted)
	if err != nil {
		t.Fatal(err)
	}
	linked, synced, marked, err := tr.follow(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if synced.GetVersion() == linked.GetVersion() || synced.GetReadHash() == linked.GetReadHash() || synced.GetNewerVersion() != "" {
		t.Errorf("after the first change, the netlist's source = %v; want it synced past %v", synced, linked)
	}
	if marked.GetVersion() != synced.GetVersion() || marked.GetNewerVersion() == "" || marked.GetNewerVersion() == synced.GetVersion() {
		t.Errorf("after the second change to a design that marks, the netlist's source = %v; want it left at %s with a newer version noted", marked, synced.GetVersion())
	}
}

// A poll reads a rule added to a linked pack's project.yaml into the pack (agni issue 1801).
func TestTheTourFollowsAPack(t *testing.T) {
	tr, err := newTour(t.TempDir(), mounted)
	if err != nil {
		t.Fatal(err)
	}
	linked, moved, err := tr.followPack(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n := len(linked.GetConfig().GetConventions().GetRules()); n != 1 || !linked.GetSource().GetFollow() {
		t.Errorf("the linked pack holds %d rules, following %v; want one, followed", n, linked.GetSource().GetFollow())
	}
	if n := len(moved.GetConfig().GetConventions().GetRules()); n != 2 || moved.GetHash() == linked.GetHash() {
		t.Errorf("after the poll the pack holds %d rules with hash %s; want two and a new hash", n, moved.GetHash())
	}
}

// A tour over a sqlite store keeps the same designs, a report on one included, and the database
// holds their records while the uploads hold the netlists (agni issue 1733).
func TestTheTourKeepsDesignsInSqlite(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	tr, recs, closeDB, err := sqliteTour(root, mounted)
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB()
	if _, _, err := tr.keep(ctx, "mixer-rev-a", "Mixer rev A", "../common/designs/rev-a.edn"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := tr.keep(ctx, "mixer-rev-b", "Mixer rev B", "../common/designs/rev-b.edn"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := tr.keepPack(ctx); err != nil {
		t.Fatal(err)
	}
	kept, read, err := tr.report(ctx)
	if err != nil || kept.GetDesignChanged() || !read.GetDesignChanged() {
		t.Fatalf("report over sqlite = %v, then %v, %v; want it current when kept and run on earlier files after", kept, read, err)
	}
	ks, err := keys(ctx, recs, "")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"designs/mixer-rev-a.yaml": true, "designs/mixer-rev-b.yaml": true, "packs/house.json": true}
	for _, k := range ks {
		delete(want, k)
	}
	if len(want) != 0 {
		t.Errorf("agni.db holds %v; missing %v", ks, want)
	}
	if blobs, _ := filepath.Glob(filepath.Join(root, "uploads", "blobs", "*", "*")); len(blobs) == 0 {
		t.Errorf("the uploads hold no blobs; the netlists should be there, not in agni.db")
	}
	if entries, _ := os.ReadDir(root); len(entries) == 0 {
		t.Fatal("the sqlite tour wrote nothing under its root")
	}
}
