package service_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/panyam/goapplib/storage/mounts"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/service"
)

// fakeAccess is a service.SourceAccess: guarded maps a mount to the private source it reads, and
// readers maps a caller's subject to the sources their own connections can read. A source listed in
// down can't answer.
type fakeAccess struct {
	guarded map[string]string
	readers map[string][]string
	down    map[string]bool
	asked   int
}

func (f *fakeAccess) Guarding(mount string) string { return f.guarded[mount] }

func (f *fakeAccess) CanRead(ctx context.Context, source string) error {
	f.asked++
	if f.down[source] {
		return fmt.Errorf("%w: %s didn't answer", service.ErrUnavailable, source)
	}
	if slices.Contains(f.readers[service.CallerFrom(ctx).Subject], source) {
		return nil
	}
	return fmt.Errorf("%w: %s is private", service.ErrPermissionDenied, source)
}

func as(subject string) context.Context {
	return service.WithCaller(context.Background(), service.Caller{Subject: subject})
}

// guardedService is a DesignService whose catalog shows a design linked from the mount "board",
// which reads github://acme/board, only to a caller who can read that repo. It holds the design bb,
// which alice linked from board, and the design notes, which has no artifact.
func guardedService(t *testing.T, access *fakeAccess) (*service.DesignService, *service.AccessGuard) {
	t.Helper()
	b := blobs{}
	board := &fakeMount{b: b, refs: map[string]string{"main": "c1"}, branches: map[string]bool{"main": true}, files: map[string]map[string]string{"c1": kicadFolder("(kicad_sch)")}}
	root := t.TempDir()
	store, err := projects.NewDirStore(filepath.Join(root, "records"), filepath.Join(root, "uploads"), "uploads")
	if err != nil {
		t.Fatal(err)
	}
	guard := service.NewAccessGuard(access, "uploads")
	svc := service.NewDesignService(service.NewDesignCatalog(store).WithGuard(guard), b.source).WithLinks(func(_ context.Context, name string) (service.MountReader, error) {
		if name == "board" {
			return board, nil
		}
		return nil, service.ErrNotFound
	}, "uploads")
	alice := as("alice")
	for _, id := range []string{"bb", "notes"} {
		if _, err := svc.CreateDesign(alice, &webapi.CreateDesignRequest{DesignId: id}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.LinkArtifact(alice, &webapi.LinkArtifactRequest{Name: "designs/bb/artifacts/schematic", Source: &webapi.ArtifactSource{Mount: "board", Path: "hw/bb.kicad_sch", Ref: "main"}, AllowMissing: true}); err != nil {
		t.Fatalf("alice links from a repo she can read: %v", err)
	}
	return svc, guard
}

func names(ds []*webapi.Design) []string {
	var out []string
	for _, d := range ds {
		out = append(out, d.GetName())
	}
	return out
}

// A design linked from a private repo opens for a caller whose own connection reads the repo, and
// for nobody else, by any call that names it (agni issue 1728).
func TestADesignLinkedFromAPrivateRepoOpensOnlyForWhoCanReadIt(t *testing.T) {
	access := &fakeAccess{guarded: map[string]string{"board": "github://acme/board"}, readers: map[string][]string{"alice": {"github://acme/board"}}}
	svc, _ := guardedService(t, access)
	alice, bob := as("alice"), as("bob")

	d, err := svc.GetDesign(alice, &webapi.GetDesignRequest{Name: "designs/bb"})
	if err != nil {
		t.Fatalf("alice GetDesign: %v", err)
	}
	if got := d.GetAccessSources(); !slices.Equal(got, []string{"github://acme/board"}) {
		t.Errorf("access_sources = %q, want the repo the schematic came from", got)
	}
	list, err := svc.ListDesigns(alice, &webapi.ListDesignsRequest{})
	if err != nil || !slices.Equal(names(list.GetDesigns()), []string{"designs/bb", "designs/notes"}) {
		t.Errorf("alice ListDesigns = %q, %v; want both", names(list.GetDesigns()), err)
	}

	denied := func(what string, err error) {
		t.Helper()
		if !errors.Is(err, service.ErrPermissionDenied) || !strings.Contains(err.Error(), "github://acme/board") {
			t.Errorf("bob %s = %v; want permission denied naming github://acme/board", what, err)
		}
	}
	_, err = svc.GetDesign(bob, &webapi.GetDesignRequest{Name: "designs/bb"})
	denied("GetDesign", err)
	_, err = svc.ListArtifacts(bob, &webapi.ListArtifactsRequest{Parent: "designs/bb"})
	denied("ListArtifacts", err)
	_, err = svc.GetArtifact(bob, &webapi.GetArtifactRequest{Name: "designs/bb/artifacts/schematic"})
	denied("GetArtifact", err)
	_, err = svc.UpdateDesign(bob, &webapi.UpdateDesignRequest{Design: &webapi.Design{Name: "designs/bb", Title: "mine now"}})
	denied("UpdateDesign", err)
	_, err = svc.SyncDesign(bob, &webapi.SyncDesignRequest{Name: "designs/bb"})
	denied("SyncDesign", err)
	denied("DeleteDesign", svc.DeleteDesign(bob, &webapi.DeleteDesignRequest{Name: "designs/bb"}))
	_, err = svc.LinkArtifact(bob, &webapi.LinkArtifactRequest{Name: "designs/notes/artifacts/schematic", Source: &webapi.ArtifactSource{Mount: "board", Path: "hw/bb.kicad_sch", Ref: "main"}, AllowMissing: true})
	denied("LinkArtifact from the repo into his own design", err)
	_, err = svc.GetDesign(context.Background(), &webapi.GetDesignRequest{Name: "designs/bb"})
	if !errors.Is(err, service.ErrPermissionDenied) {
		t.Errorf("GetDesign with no caller = %v; want permission denied", err)
	}

	list, err = svc.ListDesigns(bob, &webapi.ListDesignsRequest{})
	if err != nil || !slices.Equal(names(list.GetDesigns()), []string{"designs/notes"}) || len(list.GetUnreachable()) != 0 {
		t.Errorf("bob ListDesigns = %q, unreachable %q, %v; want notes alone", names(list.GetDesigns()), list.GetUnreachable(), err)
	}
	if d, err := svc.GetDesign(bob, &webapi.GetDesignRequest{Name: "designs/notes"}); err != nil || len(d.GetAccessSources()) != 0 {
		t.Errorf("bob GetDesign notes = %v, %v; want it, with no access source", d.GetAccessSources(), err)
	}
	if d, err := svc.GetDesign(alice, &webapi.GetDesignRequest{Name: "designs/bb"}); err != nil || d.GetTitle() == "mine now" {
		t.Errorf("after bob's refused update, alice reads %q, %v", d.GetTitle(), err)
	}
}

// A source that can't say whether the caller may read it neither opens the design nor hides it: a
// read is unavailable, and a listing names the design unreachable.
func TestADesignWhoseSourceCantAnswerIsUnreachable(t *testing.T) {
	access := &fakeAccess{guarded: map[string]string{"board": "github://acme/board"}, readers: map[string][]string{"alice": {"github://acme/board"}}}
	svc, _ := guardedService(t, access)
	access.down = map[string]bool{"github://acme/board": true}
	alice := as("alice")
	if _, err := svc.GetDesign(alice, &webapi.GetDesignRequest{Name: "designs/bb"}); !errors.Is(err, service.ErrUnavailable) {
		t.Errorf("GetDesign while GitHub is down = %v; want unavailable", err)
	}
	list, err := svc.ListDesigns(alice, &webapi.ListDesignsRequest{})
	if err != nil || !slices.Equal(names(list.GetDesigns()), []string{"designs/notes"}) || !slices.Equal(list.GetUnreachable(), []string{"designs/bb"}) {
		t.Errorf("ListDesigns while GitHub is down = %q, unreachable %q, %v; want bb unreachable", names(list.GetDesigns()), list.GetUnreachable(), err)
	}
}

// A file under the uploads mount belongs to the design its first segment names, a snapshot is
// refused when its design or any uri it names is, and a mount's own files follow the mount.
func TestTheGuardFollowsAFileOrASnapshotToItsDesign(t *testing.T) {
	access := &fakeAccess{guarded: map[string]string{"board": "github://acme/board"}, readers: map[string][]string{"alice": {"github://acme/board"}}}
	_, guard := guardedService(t, access)
	alice, bob := as("alice"), as("bob")
	for _, c := range []struct {
		uri  string
		open bool
	}{
		{"mount://uploads/bb/0123abcd/schematic/hw/bb.kicad_sch", false},
		{"mount://board/hw/bb.kicad_sch", false},
		{"mount://uploads/notes/0123abcd/schematic/x.kicad_sch", true},
		{"mount://uploads/@packs/jetson/1/symbols/a.sym", true},
		{"mount://uploads/gone/0123abcd/x.kicad_sch", true},
		{"mount://lab/hw/bb.kicad_sch", true},
	} {
		u, err := mounts.ParseURI(c.uri)
		if err != nil {
			t.Fatal(err)
		}
		if err := guard.URI(alice, u); err != nil {
			t.Errorf("alice reads %s: %v", c.uri, err)
		}
		if err := guard.URI(bob, u); (err == nil) != c.open || (err != nil && !errors.Is(err, service.ErrPermissionDenied)) {
			t.Errorf("bob reads %s: %v; want open %v", c.uri, err, c.open)
		}
	}
	for _, def := range []*webapi.Snapshot{
		{Design: "designs/bb"},
		{Uri: "mount://uploads/bb/0123abcd/schematic/hw/bb.kicad_sch"},
		{Uri: "mount://lab/x.kicad_sch", BoardUri: "mount://board/hw/bb.kicad_pcb"},
	} {
		if err := guard.Snapshot(bob, def); !errors.Is(err, service.ErrPermissionDenied) {
			t.Errorf("bob's snapshot %v = %v; want permission denied", def, err)
		}
		if err := guard.Snapshot(alice, def); err != nil {
			t.Errorf("alice's snapshot %v = %v", def, err)
		}
	}
	var none *service.AccessGuard
	if err := none.Snapshot(bob, &webapi.Snapshot{Design: "designs/bb"}); err != nil {
		t.Errorf("a nil guard, as on a server without --auth, refused %v", err)
	}
}

// A design stays guarded after its link is undone or its mount goes, since its files still came
// from the private repo, and a mount's links count every design, seen or not.
func TestAnUnlinkedOrOrphanedArtifactStaysGuarded(t *testing.T) {
	access := &fakeAccess{guarded: map[string]string{"board": "github://acme/board"}, readers: map[string][]string{"alice": {"github://acme/board"}}}
	svc, _ := guardedService(t, access)
	alice, bob := as("alice"), as("bob")

	if linked, err := svc.LinkedTo(bob, "board"); err != nil || !slices.Equal(linked, []string{"designs/bb/artifacts/schematic"}) {
		t.Errorf("LinkedTo(board) asked by bob = %q, %v; want bb's schematic, so the mount isn't deleted under it", linked, err)
	}
	a, err := svc.UnlinkArtifact(alice, &webapi.UnlinkArtifactRequest{Name: "designs/bb/artifacts/schematic"})
	if err != nil {
		t.Fatal(err)
	}
	if a.GetSource().GetAccessSource() != "github://acme/board" {
		t.Errorf("unlinked artifact's source = %v; want it to keep the repo its files came from", a.GetSource())
	}
	if _, err := svc.GetDesign(bob, &webapi.GetDesignRequest{Name: "designs/bb"}); !errors.Is(err, service.ErrPermissionDenied) {
		t.Errorf("bob GetDesign after the unlink = %v; want permission denied", err)
	}
	access.guarded = nil
	if _, err := svc.GetDesign(bob, &webapi.GetDesignRequest{Name: "designs/bb"}); !errors.Is(err, service.ErrPermissionDenied) {
		t.Errorf("bob GetDesign after the mount went = %v; want permission denied", err)
	}
	if d, err := svc.GetDesign(alice, &webapi.GetDesignRequest{Name: "designs/bb"}); err != nil || !slices.Equal(d.GetAccessSources(), []string{"github://acme/board"}) {
		t.Errorf("alice GetDesign after the mount went = %v, %v", d.GetAccessSources(), err)
	}
}
