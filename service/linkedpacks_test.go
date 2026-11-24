package service_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/panyam/goapplib/storage/mounts"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/service"
)

// rulesFolder is a project folder a team keeps its rules in: a project.yaml with one house rule per
// name in rules, and the tutorial's CAN profile.
func rulesFolder(t *testing.T, rules ...string) map[string]string {
	t.Helper()
	can, err := os.ReadFile("../examples/tutorial-project/profiles/can.yaml")
	if err != nil {
		t.Fatal(err)
	}
	y := "name: house\ntitle: House rules\nprofiles: [profiles]\nconventions:\n  name: house\n  rules:\n"
	for _, r := range rules {
		y += "    - name: " + r + "\n      severity: warning\n      allow: [\"^GND\"]\n"
	}
	return map[string]string{"rules/project.yaml": y, "rules/profiles/can.yaml": string(can), "README.md": "the team's rules"}
}

// packsOver is a PackService over a fresh folder store, linking to the mounts in readers. mountsOf
// says "rules" reads github://acme/rules.
func packsOver(t *testing.T, b blobs, readers map[string]*fakeMount, access service.SourceAccess) *service.PackService {
	t.Helper()
	root := t.TempDir()
	store, err := projects.NewDirStore(filepath.Join(root, "records"), filepath.Join(root, "uploads"), "uploads")
	if err != nil {
		t.Fatal(err)
	}
	ps, err := projects.NewDirPacks(filepath.Join(root, "packs"), store.Blobs(), projects.ViewsRoot(filepath.Join(root, "uploads")), "uploads")
	if err != nil {
		t.Fatal(err)
	}
	mountsOf := func(_ context.Context, source string) ([]string, error) {
		if source == "github://acme/rules" {
			return []string{"rules"}, nil
		}
		return nil, nil
	}
	return service.NewPackService(ps, b.source).WithImporter(projects.PackFromFolder).WithLinks(func(_ context.Context, name string) (service.MountReader, error) {
		if r, ok := readers[name]; ok {
			return r, nil
		}
		if name == "uploads" {
			return nil, service.ErrInvalidArgument
		}
		return nil, service.ErrNotFound
	}, "uploads", access, mountsOf)
}

// ruleNames lists a pack's house rules.
func ruleNames(p *webapi.Pack) []string {
	var out []string
	for _, r := range p.GetConfig().GetConventions().GetRules() {
		out = append(out, r.GetName())
	}
	return out
}

// A pack linked on a branch moves when a push names the branch, or a poll finds it moved, and a pack
// pinned at a tag stays where it was (agni issue 1801).
func TestAPackLinkedOnABranchMovesWithItAndAPinnedOneStays(t *testing.T) {
	b := blobs{}
	rules := &fakeMount{b: b, refs: map[string]string{"main": "c1", "v1": "c1"}, branches: map[string]bool{"main": true},
		files: map[string]map[string]string{"c1": rulesFolder(t, "signal-net-naming")}}
	svc := packsOver(t, b, map[string]*fakeMount{"rules": rules}, nil)
	ctx := context.Background()

	house, err := svc.LinkPack(ctx, &webapi.LinkPackRequest{Source: &webapi.ArtifactSource{Mount: "rules", Path: "rules", Ref: "main"}})
	if err != nil {
		t.Fatalf("LinkPack: %v", err)
	}
	if house.GetName() != "packs/house" || !house.GetSource().GetFollow() || house.GetSource().GetVersion() != "c1" || house.GetSource().GetReadHash() == "" {
		t.Errorf("LinkPack = %s, source %v; want packs/house following main at c1", house.GetName(), house.GetSource())
	}
	if got := ruleNames(house); !slices.Equal(got, []string{"signal-net-naming"}) {
		t.Errorf("the linked pack's rules = %v; want the folder's one", got)
	}
	if !slices.ContainsFunc(house.GetFiles(), func(f *webapi.SnapshotFile) bool { return f.GetPath() == "profiles/can.yaml" }) {
		t.Errorf("the linked pack's files = %v; want the profile project.yaml names", house.GetFiles())
	}
	pinned, err := svc.LinkPack(ctx, &webapi.LinkPackRequest{PackId: "pinned", Source: &webapi.ArtifactSource{Mount: "rules", Path: "rules", Ref: "v1"}})
	if err != nil || pinned.GetSource().GetFollow() {
		t.Fatalf("LinkPack at a tag = %v, %v; want it pinned", pinned.GetSource(), err)
	}

	rules.refs["main"] = "c2"
	rules.files["c2"] = rulesFolder(t, "signal-net-naming", "strict-net-naming")
	moves, err := svc.SourceChanged(ctx, mounts.Change{Source: "github://acme/rules", Ref: "main", RefKind: mounts.RefBranch, Version: "c2"})
	if err != nil || len(moves) != 1 || moves[0].Pack != "packs/house" || moves[0].From != "c1" || moves[0].To != "c2" {
		t.Errorf("SourceChanged for main at c2 = %+v, %v; want packs/house moved from c1 to c2", moves, err)
	}
	house, _ = svc.GetPack(ctx, &webapi.GetPackRequest{Name: "packs/house"})
	if got := ruleNames(house); house.GetSource().GetVersion() != "c2" || !slices.Equal(got, []string{"signal-net-naming", "strict-net-naming"}) {
		t.Errorf("after the push, packs/house is at %s with rules %v; want c2 with the new rule", house.GetSource().GetVersion(), got)
	}
	if p, _ := svc.GetPack(ctx, &webapi.GetPackRequest{Name: "packs/pinned"}); p.GetSource().GetVersion() != "c1" || len(ruleNames(p)) != 1 {
		t.Errorf("after the push, packs/pinned is at %s with %v; want it left at c1", p.GetSource().GetVersion(), ruleNames(p))
	}
	if again, err := svc.SourceChanged(ctx, mounts.Change{Source: "github://acme/rules", Ref: "main", RefKind: mounts.RefBranch, Version: "c2"}); err != nil || len(again) != 0 {
		t.Errorf("a redelivery of the same push = %+v, %v; want nothing moved", again, err)
	}
	if resp, err := svc.SyncPack(ctx, &webapi.SyncPackRequest{Name: "packs/house"}); err != nil || resp.GetChanged() || resp.GetToVersion() != "c2" {
		t.Errorf("SyncPack with main unmoved = %v, %v; want unchanged at c2", resp, err)
	}

	rules.refs["main"] = "c3"
	rules.files["c3"] = rulesFolder(t, "strict-net-naming")
	if moves, err := svc.PollSources(ctx); err != nil || len(moves) != 1 || moves[0].To != "c3" {
		t.Errorf("PollSources after main moved to c3 = %+v, %v; want packs/house moved", moves, err)
	}
	if got, err := svc.LinkedTo(ctx, "rules"); err != nil || !slices.Equal(got, []string{"packs/house", "packs/pinned"}) {
		t.Errorf("LinkedTo(rules) = %v, %v; want both packs, so the mount isn't deleted under them", got, err)
	}
}

// A linked pack takes its config and files from its source alone until it is unlinked, and
// unlinking keeps them as they are (agni issue 1801).
func TestALinkedPackRefusesAPutUntilUnlinked(t *testing.T) {
	b := blobs{}
	rules := &fakeMount{b: b, refs: map[string]string{"main": "c1"}, branches: map[string]bool{"main": true},
		files: map[string]map[string]string{"c1": rulesFolder(t, "signal-net-naming")}}
	svc := packsOver(t, b, map[string]*fakeMount{"rules": rules}, nil)
	ctx := context.Background()
	if _, err := svc.LinkPack(ctx, &webapi.LinkPackRequest{Source: &webapi.ArtifactSource{Mount: "rules", Path: "rules", Ref: "main"}}); err != nil {
		t.Fatal(err)
	}
	upload := b.files(map[string]string{"project.yaml": "name: house\nconventions:\n  name: house\n"})
	if _, err := svc.ImportPackFiles(ctx, &webapi.ImportPackFilesRequest{Files: upload}); !errors.Is(err, service.ErrFailedPrecondition) || !strings.Contains(err.Error(), "UnlinkPack") {
		t.Errorf("ImportPackFiles over a linked pack = %v; want FAILED_PRECONDITION saying to unlink", err)
	}
	if _, err := svc.UpdatePack(ctx, &webapi.UpdatePackRequest{Pack: &webapi.Pack{Name: "packs/house", Config: &webapi.AnalysisConfig{}}}); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("UpdatePack of a linked pack's config = %v; want FAILED_PRECONDITION", err)
	}
	if _, err := svc.LinkPack(ctx, &webapi.LinkPackRequest{Source: &webapi.ArtifactSource{Mount: "uploads", Path: "x"}}); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("LinkPack from the uploads mount = %v; want INVALID_ARGUMENT", err)
	}
	if _, err := svc.LinkPack(ctx, &webapi.LinkPackRequest{Source: &webapi.ArtifactSource{Mount: "rules", Path: "rules", Ref: "nope"}}); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("LinkPack at a ref the mount lacks = %v; want NOT_FOUND", err)
	}

	p, err := svc.UnlinkPack(ctx, &webapi.UnlinkPackRequest{Name: "packs/house"})
	if err != nil || p.GetSource().GetReadHash() != "" || !slices.Equal(ruleNames(p), []string{"signal-net-naming"}) {
		t.Errorf("UnlinkPack = %v with rules %v, %v; want no link and the same rules", p.GetSource(), ruleNames(p), err)
	}
	rules.refs["main"] = "c2"
	rules.files["c2"] = rulesFolder(t, "strict-net-naming")
	if moves, err := svc.SourceChanged(ctx, mounts.Change{Source: "github://acme/rules", Ref: "main", RefKind: mounts.RefBranch, Version: "c2"}); err != nil || len(moves) != 0 {
		t.Errorf("a push after the unlink = %+v, %v; want nothing moved", moves, err)
	}
	if _, err := svc.SyncPack(ctx, &webapi.SyncPackRequest{Name: "packs/house"}); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("SyncPack of an unlinked pack = %v; want INVALID_ARGUMENT", err)
	}
	if _, err := svc.ImportPackFiles(ctx, &webapi.ImportPackFilesRequest{Files: upload}); err != nil {
		t.Errorf("ImportPackFiles after the unlink: %v", err)
	}
}

// A pack linked from a private repo opens for a caller whose own connection reads the repo, and for
// nobody else, by name, in a listing, through its files, and after it is unlinked (agni issue 1801).
func TestALinkedPackFromAPrivateRepoOpensOnlyForItsReaders(t *testing.T) {
	b := blobs{}
	rules := &fakeMount{b: b, refs: map[string]string{"main": "c1"}, branches: map[string]bool{"main": true},
		files: map[string]map[string]string{"c1": rulesFolder(t, "signal-net-naming")}}
	access := &fakeAccess{guarded: map[string]string{"rules": "github://acme/rules"}, readers: map[string][]string{"alice": {"github://acme/rules"}}}
	guard := service.NewAccessGuard(access, "uploads")
	svc := packsOver(t, b, map[string]*fakeMount{"rules": rules}, access).WithGuard(guard)
	alice, bob := as("alice"), as("bob")

	house, err := svc.LinkPack(alice, &webapi.LinkPackRequest{Source: &webapi.ArtifactSource{Mount: "rules", Path: "rules", Ref: "main"}})
	if err != nil {
		t.Fatalf("alice links from a repo she can read: %v", err)
	}
	if got := house.GetSource().GetAccessSource(); got != "github://acme/rules" {
		t.Errorf("the pack's access source = %q; want the repo", got)
	}
	if _, err := svc.GetPack(alice, &webapi.GetPackRequest{Name: "packs/house"}); err != nil {
		t.Errorf("alice GetPack: %v", err)
	}
	denied := func(what string, err error) {
		t.Helper()
		if !errors.Is(err, service.ErrPermissionDenied) || !strings.Contains(err.Error(), "github://acme/rules") {
			t.Errorf("bob %s = %v; want permission denied naming github://acme/rules", what, err)
		}
	}
	_, err = svc.GetPack(bob, &webapi.GetPackRequest{Name: "packs/house"})
	denied("GetPack", err)
	_, err = service.ResolvePacks(bob, svc, []string{"packs/house"})
	denied("ResolvePacks, as a check under the pack reads it", err)
	_, err = svc.SyncPack(bob, &webapi.SyncPackRequest{Name: "packs/house"})
	denied("SyncPack", err)
	denied("DeletePack", svc.DeletePack(bob, &webapi.DeletePackRequest{Name: "packs/house"}))
	_, err = svc.LinkPack(bob, &webapi.LinkPackRequest{PackId: "mine", Source: &webapi.ArtifactSource{Mount: "rules", Path: "rules", Ref: "main"}})
	denied("LinkPack from the repo", err)
	files, err := mounts.ParseURI(house.GetUri() + "/profiles/can.yaml")
	if err != nil {
		t.Fatal(err)
	}
	denied("reading the pack's profile by its URI", guard.URI(bob, files))
	if err := guard.URI(alice, files); err != nil {
		t.Errorf("alice reading the pack's profile by its URI = %v; want it read", err)
	}
	if list, err := svc.ListPacks(bob, &webapi.ListPacksRequest{}); err != nil || len(list.GetPacks()) != 0 {
		t.Errorf("bob ListPacks = %v, %v; want the pack left out", list.GetPacks(), err)
	}
	if list, err := svc.ListPacks(alice, &webapi.ListPacksRequest{}); err != nil || len(list.GetPacks()) != 1 {
		t.Errorf("alice ListPacks = %v, %v; want the pack", list.GetPacks(), err)
	}

	if _, err := svc.UnlinkPack(alice, &webapi.UnlinkPackRequest{Name: "packs/house"}); err != nil {
		t.Fatal(err)
	}
	_, err = svc.GetPack(bob, &webapi.GetPackRequest{Name: "packs/house"})
	denied("GetPack after alice unlinked it", err)
}
