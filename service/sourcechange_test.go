package service_test

import (
	"context"
	"slices"
	"testing"

	"github.com/panyam/goapplib/storage/mounts"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// linkIn links artifact schematic of design to path in mount at ref, creating the design.
func linkIn(t *testing.T, svc *service.DesignService, design, mount, ref string) {
	t.Helper()
	ctx := context.Background()
	if _, err := svc.GetDesign(ctx, &webapi.GetDesignRequest{Name: "designs/" + design}); err != nil {
		if _, err := svc.CreateDesign(ctx, &webapi.CreateDesignRequest{DesignId: design}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.LinkArtifact(ctx, &webapi.LinkArtifactRequest{
		Name: "designs/" + design + "/artifacts/schematic", Source: &webapi.ArtifactSource{Mount: mount, Path: "hw/bb.kicad_sch", Ref: ref}, AllowMissing: true,
	}); err != nil {
		t.Fatal(err)
	}
}

// sourceOf is the source of design's schematic.
func sourceOf(t *testing.T, svc *service.DesignService, design string) *webapi.ArtifactSource {
	t.Helper()
	a, err := svc.GetArtifact(context.Background(), &webapi.GetArtifactRequest{Name: "designs/" + design + "/artifacts/schematic"})
	if err != nil {
		t.Fatal(err)
	}
	return a.GetSource()
}

// A push moves every design whose links follow the pushed branch, through any mount of the repo,
// and leaves the rest alone. A design set to mark notes the new commit instead, and so does one whose
// source can't be read with no one viewing (agni issue 1726).
func TestAPushMovesTheDesignsThatFollowItsBranch(t *testing.T) {
	ctx := context.Background()
	b := blobs{}
	files := map[string]map[string]string{"c1": kicadFolder("(kicad_sch v1)"), "c2": kicadFolder("(kicad_sch v2)"), "d1": kicadFolder("(kicad_sch dev)")}
	refs := func() map[string]string { return map[string]string{"": "c1", "main": "c1", "dev": "d1", "v1": "c1"} }
	board := &fakeMount{b: b, refs: refs(), branches: map[string]bool{"main": true, "dev": true}, files: files}
	mine := &fakeMount{b: b, refs: refs(), branches: map[string]bool{"main": true, "dev": true}, files: files}
	other := &fakeMount{b: b, refs: refs(), branches: map[string]bool{"main": true}, files: files}
	svc, _, _ := linkedService(t, b, map[string]*fakeMount{"board": board, "mine": mine, "other": other})
	svc.WithSourceChanges(func(_ context.Context, source string) ([]string, error) {
		if source == "github://acme/board" {
			return []string{"board", "mine"}, nil
		}
		return nil, nil
	})
	linkIn(t, svc, "bb", "board", "main")
	linkIn(t, svc, "bydefault", "board", "")
	linkIn(t, svc, "dv", "board", "dev")
	linkIn(t, svc, "pin", "board", "v1")
	linkIn(t, svc, "mk", "board", "main")
	linkIn(t, svc, "pv", "mine", "main")
	linkIn(t, svc, "elsewhere", "other", "main")
	if _, err := svc.UpdateDesign(ctx, &webapi.UpdateDesignRequest{
		Design: &webapi.Design{Name: "designs/mk", OnSourceChange: webapi.SourceChange_SOURCE_CHANGE_MARK}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"on_source_change"}},
	}); err != nil {
		t.Fatal(err)
	}
	if d, err := svc.GetDesign(ctx, &webapi.GetDesignRequest{Name: "designs/mk"}); err != nil || d.GetOnSourceChange() != webapi.SourceChange_SOURCE_CHANGE_MARK {
		t.Fatalf("designs/mk's on_source_change = %v, %v; want MARK", d.GetOnSourceChange(), err)
	}

	for _, m := range []*fakeMount{board, mine, other} {
		m.refs["main"], m.refs[""] = "c2", "c2"
	}
	// A person's own connection can't be read with no one viewing.
	mine.err = service.ErrFailedPrecondition
	push := mounts.Change{Source: "github://acme/board", Ref: "main", RefKind: mounts.RefBranch, Default: true, Version: "c2", Before: "c1"}
	moves, err := svc.SourceChanged(ctx, push)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]service.SourceMove{}
	for _, m := range moves {
		got[m.Design] = m
	}
	if len(moves) != 4 {
		t.Errorf("a push to main moved %v; want bb, bydefault, mk and pv", moves)
	}
	for _, d := range []string{"bb", "bydefault"} {
		if m := got["designs/"+d]; !m.Synced || !slices.Equal(m.Artifacts, []string{"schematic"}) {
			t.Errorf("a push to main did %+v to %s; want its schematic synced", m, d)
		}
		if src := sourceOf(t, svc, d); src.GetVersion() != "c2" || src.GetNewerVersion() != "" {
			t.Errorf("%s's schematic after the push = %v; want it at c2", d, src)
		}
	}
	if m := got["designs/mk"]; m.Synced || m.Err != nil {
		t.Errorf("a push to main did %+v to mk; want it marked, since mk marks", m)
	}
	if src := sourceOf(t, svc, "mk"); src.GetVersion() != "c1" || src.GetNewerVersion() != "c2" || src.GetNewerTime() == nil {
		t.Errorf("mk's schematic after the push = %v; want it at c1, with c2 newer at the source", src)
	}
	if m := got["designs/pv"]; m.Synced || m.Err == nil {
		t.Errorf("a push to main did %+v to pv; want its sync to fail and the design marked", m)
	}
	if src := sourceOf(t, svc, "pv"); src.GetVersion() != "c1" || src.GetNewerVersion() != "c2" {
		t.Errorf("pv's schematic after the push = %v; want it at c1, with c2 newer at the source", src)
	}
	for d, want := range map[string]string{"dv": "d1", "pin": "c1", "elsewhere": "c1"} {
		if src := sourceOf(t, svc, d); src.GetVersion() != want || src.GetNewerVersion() != "" {
			t.Errorf("%s's schematic after a push to main = %v; want it left at %s", d, src, want)
		}
	}

	// GitHub sends a delivery again when it doesn't hear back, and that one reads nothing.
	resolves := board.resolves + mine.resolves
	if again, err := svc.SourceChanged(ctx, push); err != nil || len(again) != 0 || board.resolves+mine.resolves != resolves {
		t.Errorf("a redelivery moved %v, %v, resolving %d; want nothing", again, err, board.resolves+mine.resolves-resolves)
	}

	if _, err := svc.SyncDesign(ctx, &webapi.SyncDesignRequest{Name: "designs/mk"}); err != nil {
		t.Fatal(err)
	}
	if src := sourceOf(t, svc, "mk"); src.GetVersion() != "c2" || src.GetNewerVersion() != "" {
		t.Errorf("mk's schematic after a sync = %v; want it at c2 with no mark", src)
	}

	// A late delivery of an older push names c1, and main is at c2, which mk and bb hold.
	late := push
	late.Version, late.Before = "c1", "c0"
	if moves, err := svc.SourceChanged(ctx, late); err != nil || len(moves) != 0 {
		t.Errorf("a late push naming c1 moved %+v, %v; want nothing, since main is at c2", moves, err)
	}
	if src := sourceOf(t, svc, "mk"); src.GetVersion() != "c2" || src.GetNewerVersion() != "" {
		t.Errorf("mk's schematic after a late push naming c1 = %v; want it at c2 with no mark", src)
	}

	board.refs["dev"] = "c2"
	moves, err = svc.SourceChanged(ctx, mounts.Change{Source: "github://acme/board", Ref: "dev", RefKind: mounts.RefBranch, Version: "c2"})
	if err != nil || len(moves) != 1 || moves[0].Design != "designs/dv" || !moves[0].Synced {
		t.Errorf("a push to dev moved %+v, %v; want dv alone", moves, err)
	}
	if moves, err := svc.SourceChanged(ctx, mounts.Change{Source: "github://acme/board", Ref: "dev", RefKind: mounts.RefBranch, Deleted: true}); err != nil || len(moves) != 0 {
		t.Errorf("deleting dev moved %+v, %v; want nothing", moves, err)
	}
}

// A poll asks each source a design follows once per pass, however many designs follow it, and
// syncs the designs behind it (agni issue 1726).
func TestAPollAsksEachSourceOnceAndSyncsWhatMoved(t *testing.T) {
	ctx := context.Background()
	b := blobs{}
	board := &fakeMount{b: b, refs: map[string]string{"main": "c1", "v1": "c1"}, branches: map[string]bool{"main": true},
		files: map[string]map[string]string{"c1": kicadFolder("(kicad_sch v1)"), "c2": kicadFolder("(kicad_sch v2)")}}
	svc, _, _ := linkedService(t, b, map[string]*fakeMount{"board": board})
	for _, d := range []string{"a", "b", "c"} {
		linkIn(t, svc, d, "board", "main")
	}
	linkIn(t, svc, "pin", "board", "v1")

	resolves := board.resolves
	if moves, err := svc.PollSources(ctx); err != nil || len(moves) != 0 || board.resolves-resolves != 1 {
		t.Errorf("a poll with nothing moved = %v, %v, resolving %d times; want nothing moved, after one resolve of main", moves, err, board.resolves-resolves)
	}

	board.refs["main"] = "c2"
	moves, err := svc.PollSources(ctx)
	if err != nil || len(moves) != 3 {
		t.Fatalf("a poll after main moved = %+v, %v; want a, b and c synced", moves, err)
	}
	for _, m := range moves {
		if !m.Synced {
			t.Errorf("a poll after main moved did %+v; want it synced", m)
		}
	}
	for d, want := range map[string]string{"a": "c2", "b": "c2", "c": "c2", "pin": "c1"} {
		if v := sourceOf(t, svc, d).GetVersion(); v != want {
			t.Errorf("%s's schematic after the poll is at %s; want %s", d, v, want)
		}
	}
}
