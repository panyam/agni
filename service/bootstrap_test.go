package service_test

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/panyam/goapplib/storage/connections/fsstore"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/service"
)

// bootWorld is a server's design, pack and project services over fresh stores. The mount "board"
// reads a private repo only alice may read, and bootstrap runs with no caller at all.
type bootWorld struct {
	boot    *service.Bootstrap
	board   *fakeMount
	designs *service.DesignService
	// catalog writes the store's records directly, as an older server left them.
	catalog *service.DesignCatalog
	blobs   blobs
}

func bootstrapWorld(t *testing.T) *bootWorld {
	t.Helper()
	b := blobs{}
	board := &fakeMount{b: b, refs: map[string]string{"main": "c1", "rev-a": "c0"}, branches: map[string]bool{"main": true},
		files: map[string]map[string]string{"c1": kicadFolder("(kicad_sch)"), "c0": kicadFolder("(kicad_sch (old))")}}
	rules := &fakeMount{b: b, refs: map[string]string{"main": "r1"}, branches: map[string]bool{"main": true},
		files: map[string]map[string]string{"r1": rulesFolder(t, "house-naming")}}
	access := &fakeAccess{guarded: map[string]string{"board": "github://acme/board"}, readers: map[string][]string{"alice": {"github://acme/board"}}}
	guard := service.NewAccessGuard(access, "uploads")
	root := t.TempDir()
	store, err := projects.NewDirStore(filepath.Join(root, "records"), filepath.Join(root, "uploads"), "uploads")
	if err != nil {
		t.Fatal(err)
	}
	catalog := service.NewDesignCatalog(store).WithGuard(guard)
	designs := service.NewDesignService(catalog, b.source).WithLinks(func(_ context.Context, name string) (service.MountReader, error) {
		if name == "board" {
			return board, nil
		}
		return nil, service.ErrNotFound
	}, "uploads")
	packs := packsOver(t, b, map[string]*fakeMount{"rules": rules}, access).WithGuard(guard)
	projectSvc := service.NewProjectService(nil).WithRecords(service.NewMemProjectRecords())
	return &bootWorld{
		boot:  &service.Bootstrap{Designs: designs, Packs: packs, Projects: projectSvc},
		board: board, designs: designs, catalog: catalog, blobs: b,
	}
}

// bootPlan is a pack from the rules repo, the design bb from the private board repo at ref, read
// under the pack, and a project grouping them.
func bootPlan(t *testing.T, ref string) *service.BootstrapPlan {
	t.Helper()
	p, err := service.ParseBootstrap([]byte(`
packs:
  - id: house
    link: rules/rules@main
designs:
  - id: bb
    title: Baseboard
    packs: [house]
    link:
      schematic: board/hw/bb.kicad_sch@` + ref + `
projects:
  - id: team
    designs: [bb]
    packs: [house]
`))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func failures(r *service.BootstrapResult) []string {
	var out []string
	for _, f := range r.Failed {
		out = append(out, f.Name+": "+f.Err.Error())
	}
	return out
}

// A server with nothing in it gets every entry the plan names, links from a private repo with no
// caller to ask about, and a second apply writes nothing (agni issue 1808).
func TestBootstrapAppliesAPlanAndASecondApplyWritesNothing(t *testing.T) {
	w := bootstrapWorld(t)
	ctx := context.Background()
	r := w.boot.Apply(ctx, bootPlan(t, "main"))
	if !r.OK() {
		t.Fatalf("first apply failed: %v", failures(r))
	}
	if want := []string{"packs/house", "designs/bb", "projects/team"}; !slices.Equal(r.Applied, want) {
		t.Errorf("first apply made %v; want %v", r.Applied, want)
	}
	a, err := w.designs.GetArtifact(as("alice"), &webapi.GetArtifactRequest{Name: "designs/bb/artifacts/schematic"})
	if err != nil || len(a.GetFiles()) != 3 || a.GetSource().GetRef() != "main" {
		t.Fatalf("designs/bb's schematic as alice = %v files at %q, %v; want the root sheet, its child and the board at main", len(a.GetFiles()), a.GetSource().GetRef(), err)
	}
	if _, err := w.designs.GetArtifact(as("mallory"), &webapi.GetArtifactRequest{Name: "designs/bb/artifacts/schematic"}); !errors.Is(err, service.ErrPermissionDenied) {
		t.Errorf("designs/bb's schematic as mallory: %v; want permission denied, as for any design linked from the repo", err)
	}
	if d, err := w.designs.GetDesign(as("alice"), &webapi.GetDesignRequest{Name: "designs/bb"}); err != nil || !slices.Equal(d.GetPacks(), []string{"packs/house"}) || d.GetTitle() != "Baseboard" {
		t.Errorf("designs/bb = %v, %v; want the title Baseboard, read under packs/house", d, err)
	}

	fetched := len(w.board.fetches)
	again := w.boot.Apply(ctx, bootPlan(t, "main"))
	if !again.OK() || len(again.Applied) != 0 {
		t.Errorf("second apply = applied %v, failed %v; want nothing applied", again.Applied, failures(again))
	}
	if want := []string{"packs/house", "designs/bb", "projects/team"}; !slices.Equal(again.Unchanged, want) {
		t.Errorf("second apply left %v unchanged; want %v", again.Unchanged, want)
	}
	if len(w.board.fetches) != fetched {
		t.Errorf("second apply fetched %v; want no file read again", w.board.fetches[fetched:])
	}
}

// A design whose link names another ref is linked again at it.
func TestBootstrapLinksAgainWhenTheRefChanges(t *testing.T) {
	w := bootstrapWorld(t)
	ctx := context.Background()
	if r := w.boot.Apply(ctx, bootPlan(t, "main")); !r.OK() {
		t.Fatal(failures(r))
	}
	r := w.boot.Apply(ctx, bootPlan(t, "rev-a"))
	if !r.OK() || !slices.Equal(r.Applied, []string{"designs/bb"}) {
		t.Fatalf("apply at rev-a = applied %v, failed %v; want designs/bb alone", r.Applied, failures(r))
	}
	a, err := w.designs.GetArtifact(as("alice"), &webapi.GetArtifactRequest{Name: "designs/bb/artifacts/schematic"})
	if err != nil || a.GetSource().GetRef() != "rev-a" || a.GetSource().GetVersion() != "c0" {
		t.Errorf("designs/bb's schematic after the plan moved to rev-a = %v at %q (%q), %v; want rev-a at c0", a.GetName(), a.GetSource().GetRef(), a.GetSource().GetVersion(), err)
	}
}

// A design made by hand is reported while the plan doesn't name it, and deleted only when the plan
// says prune.
func TestBootstrapReportsWhatThePlanDoesNotNameAndPrunesOnlyWhenAsked(t *testing.T) {
	w := bootstrapWorld(t)
	ctx := context.Background()
	if _, err := w.designs.CreateDesign(ctx, &webapi.CreateDesignRequest{DesignId: "alice-test"}); err != nil {
		t.Fatal(err)
	}
	plan := bootPlan(t, "main")
	r := w.boot.Apply(ctx, plan)
	if !r.OK() || !slices.Equal(r.Unnamed, []string{"designs/alice-test"}) || len(r.Pruned) != 0 {
		t.Fatalf("apply = unnamed %v, pruned %v, failed %v; want designs/alice-test reported and kept", r.Unnamed, r.Pruned, failures(r))
	}
	if _, err := w.designs.GetDesign(ctx, &webapi.GetDesignRequest{Name: "designs/alice-test"}); err != nil {
		t.Fatalf("designs/alice-test after an apply without prune: %v", err)
	}
	plan.Prune = true
	r = w.boot.Apply(ctx, plan)
	if !r.OK() || !slices.Equal(r.Pruned, []string{"designs/alice-test"}) {
		t.Fatalf("apply with prune = pruned %v, failed %v; want designs/alice-test", r.Pruned, failures(r))
	}
	if _, err := w.designs.GetDesign(ctx, &webapi.GetDesignRequest{Name: "designs/alice-test"}); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("designs/alice-test after prune: %v; want not found", err)
	}
	if _, err := w.designs.GetDesign(as("alice"), &webapi.GetDesignRequest{Name: "designs/bb"}); err != nil {
		t.Errorf("designs/bb after prune: %v; want it kept", err)
	}
}

// One entry that can't apply leaves the rest applied and prunes nothing, since the failed entry
// might have kept what a prune would delete.
func TestBootstrapKeepsGoingPastAFailedEntryAndThenPrunesNothing(t *testing.T) {
	w := bootstrapWorld(t)
	ctx := context.Background()
	if _, err := w.designs.CreateDesign(ctx, &webapi.CreateDesignRequest{DesignId: "alice-test"}); err != nil {
		t.Fatal(err)
	}
	plan := bootPlan(t, "main")
	plan.Prune = true
	plan.Designs = append(plan.Designs, service.BootstrapDesign{ID: "psu", Link: map[string]string{"schematic": "nowhere/psu.kicad_sch"}})
	r := w.boot.Apply(ctx, plan)
	if len(r.Failed) != 1 || r.Failed[0].Name != "designs/psu" || !errors.Is(r.Failed[0].Err, service.ErrNotFound) {
		t.Errorf("apply = failed %v; want designs/psu alone, not found", failures(r))
	}
	if want := []string{"packs/house", "designs/bb", "projects/team"}; !slices.Equal(r.Applied, want) {
		t.Errorf("apply made %v; want %v despite the failure", r.Applied, want)
	}
	if len(r.Unnamed) != 0 || len(r.Pruned) != 0 {
		t.Errorf("apply with a failure = unnamed %v, pruned %v; want neither looked at", r.Unnamed, r.Pruned)
	}
	if _, err := w.designs.GetDesign(ctx, &webapi.GetDesignRequest{Name: "designs/alice-test"}); err != nil {
		t.Errorf("designs/alice-test after a failed apply: %v", err)
	}
}

// A typo in the file is refused with every problem named, and an empty file is an empty plan.
func TestParseBootstrapRefusesWhatThePlanCannotMean(t *testing.T) {
	for _, tc := range []struct{ yaml, want string }{
		{"design:\n  - id: bb\n", "field design not found"},
		{"packs:\n  - id: house\n    link: rules\n  - id: house\n    link: rules\n", "packs/house is declared twice"},
		{"packs:\n  - link: rules\n", "packs entry 1 has no id"},
		{"designs:\n  - id: bb\n    link: {schematic: \"@main\"}\n", "names no mount"},
		{"designs:\n  - id: bb\n", "designs/bb links no artifact"},
		{"connections:\n  - id: hw\n    kind: github-user\n", "kind \"github-user\" must be"},
		{"mounts:\n  - id: board\n", "mounts/board names no source"},
		{"designs:\n  - id: BB\n    link: {schematic: board}\n", "designs/BB: the id must be"},
	} {
		_, err := service.ParseBootstrap([]byte(tc.yaml))
		if err == nil || !errors.Is(err, service.ErrInvalidArgument) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("ParseBootstrap(%q) = %v; want invalid argument naming %q", tc.yaml, err, tc.want)
		}
	}
	p, err := service.ParseBootstrap(nil)
	if err != nil || p == nil || len(p.Designs) != 0 {
		t.Errorf("ParseBootstrap of an empty file = %v, %v; want an empty plan", p, err)
	}
}

// A connection the plan names is made once, left alone while it matches, and made again under its
// id when the plan names another secret. A connection made by hand is never pruned.
func TestBootstrapMakesAConnectionAndRemakesItWhenThePlanChangesIt(t *testing.T) {
	store, err := fsstore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	conns := service.NewConnectionService(store, nil)
	ctx := context.Background()
	if _, err := conns.CreateConnection(ctx, &webapi.CreateConnectionRequest{ConnectionId: "by-hand", Connection: &webapi.Connection{Kind: webapi.ConnectionKind_CONNECTION_KIND_GITHUB_TOKEN, TokenSource: "env:BY_HAND"}}); err != nil {
		t.Fatal(err)
	}
	boot := &service.Bootstrap{Connections: conns}
	plan := func(token string) *service.BootstrapPlan {
		p, err := service.ParseBootstrap([]byte("prune: true\nconnections:\n  - id: hw\n    kind: github-token\n    token: " + token + "\n"))
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	steps := []struct {
		token, applied, unchanged string
	}{
		{"env:HW_TOKEN", "connections/hw", ""},
		{"env:HW_TOKEN", "", "connections/hw"},
		{"file:/run/secrets/hw", "connections/hw", ""},
	}
	for _, s := range steps {
		r := boot.Apply(ctx, plan(s.token))
		if !r.OK() || strings.Join(r.Applied, ",") != s.applied || strings.Join(r.Unchanged, ",") != s.unchanged || len(r.Pruned) != 0 {
			t.Errorf("apply with token %s = applied %v, unchanged %v, pruned %v, failed %v; want applied %q, unchanged %q", s.token, r.Applied, r.Unchanged, r.Pruned, failures(r), s.applied, s.unchanged)
		}
	}
	c, err := conns.GetConnection(ctx, &webapi.GetConnectionRequest{Name: "connections/hw"})
	if err != nil || c.GetTokenSource() != "file:/run/secrets/hw" {
		t.Errorf("connections/hw = %q, %v; want the plan's last token source", c.GetTokenSource(), err)
	}
	if _, err := conns.GetConnection(ctx, &webapi.GetConnectionRequest{Name: "connections/by-hand"}); err != nil {
		t.Errorf("connections/by-hand after a pruning apply: %v; want it kept", err)
	}
}
