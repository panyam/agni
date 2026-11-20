package service_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// A linked design reads at a tag as a revision of its own: the files at that commit, fetched only
// where the server lacks them, with the design's own files left where they were. A second read of
// the tag asks the source nothing, and a branch is read afresh each time (agni issue 1725).
func TestADesignReadsAtARefWithoutMovingIt(t *testing.T) {
	ctx := context.Background()
	b := blobs{}
	board := &fakeMount{b: b, refs: map[string]string{"main": "c2", "1.1.2": "c1", "1.1.3": "c2"}, branches: map[string]bool{"main": true},
		files: map[string]map[string]string{"c1": kicadFolder("(kicad_sch v1)"), "c2": kicadFolder("(kicad_sch v2)")}}
	svc, _, _ := linkedService(t, b, map[string]*fakeMount{"board": board})
	link(t, svc, "schematic", "board", "hw/bb.kicad_sch", "main")
	now, err := svc.GetDesign(ctx, &webapi.GetDesignRequest{Name: "designs/bb"})
	if err != nil {
		t.Fatal(err)
	}
	board.fetches = nil

	old, err := svc.GetDesign(ctx, &webapi.GetDesignRequest{Name: "designs/bb@1.1.2"})
	if err != nil {
		t.Fatalf("designs/bb@1.1.2: %v", err)
	}
	if !strings.HasPrefix(old.GetName(), "designs/bb@") || old.GetEntryUri() == now.GetEntryUri() || !strings.HasPrefix(old.GetEntryUri(), "mount://uploads/bb/") {
		t.Errorf("designs/bb@1.1.2 = %s reading %s; want a revision of bb reading its own view, apart from %s", old.GetName(), old.GetEntryUri(), now.GetEntryUri())
	}
	if !slices.Equal(board.fetches, []string{"hw/power.kicad_sch"}) {
		t.Errorf("reading 1.1.2 fetched %v; want the one sheet that differs", board.fetches)
	}
	arts, err := svc.ListArtifacts(ctx, &webapi.ListArtifactsRequest{Parent: "designs/bb"})
	if err != nil || len(arts.GetArtifacts()) != 1 || arts.GetArtifacts()[0].GetSource().GetVersion() != "c2" {
		t.Errorf("after reading 1.1.2 the design's schematic = %v, %v; want it still at c2", arts.GetArtifacts(), err)
	}

	resolves := board.resolves
	if again, err := svc.GetDesign(ctx, &webapi.GetDesignRequest{Name: "designs/bb@1.1.2"}); err != nil || again.GetEntryUri() != old.GetEntryUri() || board.resolves != resolves || len(board.fetches) != 1 {
		t.Errorf("a second read of 1.1.2 = %v, %v, resolving %d more and fetching %v; want the kept revision with no call to the source", again.GetEntryUri(), err, board.resolves-resolves, board.fetches)
	}
	if head, err := svc.GetDesign(ctx, &webapi.GetDesignRequest{Name: "designs/bb@main"}); err != nil || head.GetEntryUri() != now.GetEntryUri() || board.resolves == resolves {
		t.Errorf("designs/bb@main = %v, %v; want the branch resolved again, giving the files the design holds", head.GetEntryUri(), err)
	}
	if _, err := svc.GetDesign(ctx, &webapi.GetDesignRequest{Name: "designs/bb@1.1.9"}); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("designs/bb@1.1.9 = %v; want not found", err)
	}
	if _, err := svc.GetDesign(ctx, &webapi.GetDesignRequest{Name: "designs/bb@"}); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("designs/bb@ = %v; want invalid argument", err)
	}

	list, err := svc.ListDesignRevisions(ctx, &webapi.ListDesignRevisionsRequest{Parent: "designs/bb"})
	if err != nil {
		t.Fatal(err)
	}
	var tagged, current int
	for _, r := range list.GetRevisions() {
		if slices.Equal(r.GetRefs(), []string{"1.1.2"}) && r.GetSourceVersions()["schematic"] == "c1" && r.GetName() == old.GetName() {
			tagged++
		}
		if r.GetCurrent() && r.GetSourceVersions()["schematic"] == "c2" && len(r.GetRefs()) == 0 {
			current++
		}
	}
	if len(list.GetRevisions()) != 2 || tagged != 1 || current != 1 {
		t.Errorf("revisions = %v; want the 1.1.2 read, and the current one with no branch name kept on it", list.GetRevisions())
	}

	if _, err := svc.CreateDesign(ctx, &webapi.CreateDesignRequest{DesignId: "up"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateArtifact(ctx, &webapi.UpdateArtifactRequest{AllowMissing: true, Artifact: &webapi.Artifact{Name: "designs/up/artifacts/schematic", Files: b.files(map[string]string{"x.kicad_sch": "(kicad_sch)"})}}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetDesign(ctx, &webapi.GetDesignRequest{Name: "designs/up@1.1.2"}); !errors.Is(err, service.ErrNotFound) || !strings.Contains(err.Error(), "no artifact of it links to a source with history") {
		t.Errorf("an uploaded design at a ref = %v; want not found, saying nothing links", err)
	}
}

// ListRefs lists the fake's tags, or with commits, one commit per tag on the branch asked.
func (m *fakeMount) ListRefs(_ context.Context, commits bool, branch string, _ int, _ string) ([]service.SourceRef, string, error) {
	if commits {
		m.branchAsked = branch
	}
	return m.tags, "", nil
}

// A linked design lists its repo's tags as revisions, newest first, read or not, and a tag the
// server has read carries its revision (agni issue 1725).
func TestARevisionListingNamesTheReposTags(t *testing.T) {
	ctx := context.Background()
	b := blobs{}
	t1, t2, t3 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	board := &fakeMount{b: b, refs: map[string]string{"main": "c2", "1.1.2": "c1", "1.1.3": "c2"}, branches: map[string]bool{"main": true},
		files: map[string]map[string]string{"c1": kicadFolder("(kicad_sch v1)"), "c2": kicadFolder("(kicad_sch v2)")},
		tags:  []service.SourceRef{{Name: "1.1.4", Version: "c3", Time: t3}, {Name: "1.1.3", Version: "c2", Time: t2, Message: "1.1.3"}, {Name: "1.1.2", Version: "c1", Time: t1}}}
	svc, _, _ := linkedService(t, b, map[string]*fakeMount{"board": board})
	link(t, svc, "schematic", "board", "hw/bb.kicad_sch", "main")
	read, err := svc.GetDesign(ctx, &webapi.GetDesignRequest{Name: "designs/bb@1.1.2"})
	if err != nil {
		t.Fatal(err)
	}

	list, err := svc.ListDesignRevisions(ctx, &webapi.ListDesignRevisionsRequest{Parent: "designs/bb", Source: webapi.DesignRevisionSource_DESIGN_REVISION_SOURCE_TAGS})
	if err != nil {
		t.Fatal(err)
	}
	revs := list.GetRevisions()
	if len(revs) != 3 {
		t.Fatalf("tags = %v; want 1.1.4, 1.1.3 and 1.1.2", revs)
	}
	if r := revs[0]; r.GetName() != "designs/bb@1.1.4" || r.GetRevisionId() != "" || r.GetSourceVersions()["schematic"] != "c3" {
		t.Errorf("newest tag = %v; want 1.1.4 at c3, not read yet", r)
	}
	revs = revs[1:]
	if r := revs[0]; r.GetName() != "designs/bb@1.1.3" || r.GetRevisionId() == "" || !r.GetCurrent() || r.GetSourceVersions()["schematic"] != "c2" || !r.GetCommitTime().AsTime().Equal(t2) || r.GetMessage() != "1.1.3" {
		t.Errorf("first tag = %v; want 1.1.3 at c2, the commit the design holds, so read and current, with its date and message", r)
	}
	if r := revs[1]; r.GetName() != "designs/bb@1.1.2" || "designs/bb@"+r.GetRevisionId() != read.GetName() || r.GetSourceVersions()["schematic"] != "c1" {
		t.Errorf("second tag = %v; want 1.1.2 at c1, carrying the revision read as %s", r, read.GetName())
	}

	if _, err := svc.ListDesignRevisions(ctx, &webapi.ListDesignRevisionsRequest{Parent: "designs/bb", Source: webapi.DesignRevisionSource_DESIGN_REVISION_SOURCE_COMMITS}); err != nil || board.branchAsked != "main" {
		t.Errorf("commits listed the branch %q, %v; want main, the branch the schematic follows", board.branchAsked, err)
	}
	if _, err := svc.ListDesignRevisions(ctx, &webapi.ListDesignRevisionsRequest{Parent: "designs/bb", Source: webapi.DesignRevisionSource_DESIGN_REVISION_SOURCE_COMMITS, Branch: "dev"}); err != nil || board.branchAsked != "dev" {
		t.Errorf("commits with branch dev listed %q, %v", board.branchAsked, err)
	}

	other := &fakeMount{b: b, refs: map[string]string{"main": "d1"}, branches: map[string]bool{"main": true}, files: map[string]map[string]string{"d1": {"pcb/bb.kicad_pcb": "(kicad_pcb)"}}}
	svc2, _, _ := linkedService(t, b, map[string]*fakeMount{"board": board, "other": other})
	link(t, svc2, "schematic", "board", "hw/bb.kicad_sch", "main")
	link(t, svc2, "board", "other", "pcb/bb.kicad_pcb", "main")
	if _, err := svc2.ListDesignRevisions(ctx, &webapi.ListDesignRevisionsRequest{Parent: "designs/bb", Source: webapi.DesignRevisionSource_DESIGN_REVISION_SOURCE_TAGS}); !errors.Is(err, service.ErrInvalidArgument) || !strings.Contains(err.Error(), "two repos") {
		t.Errorf("tags of a design linking two repos = %v; want invalid argument naming both", err)
	}
}
