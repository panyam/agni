package projects

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/records/recordstest"
	"github.com/panyam/agni/service"
)

// dirPacks is a DirPacks sharing a fresh DirStore's blobs and views, as `agni serve` builds it.
func dirPacks(t *testing.T) (*DirPacks, *DirStore, layout) {
	t.Helper()
	ds, l := dirStore(t)
	ps := NewPacks(recordstest.New(t), ds.Blobs(), ViewsRoot(l.uploads), "uploads")
	return ps, ds, l
}

var packURI = regexp.MustCompile(`^mount://uploads/@packs/house/[0-9a-f]{12}$`)

// A pack's files are linked into a view under the uploads mount, and its config's relative paths
// read from there (agni issue 1704).
func TestAPackKeepsItsFilesInAView(t *testing.T) {
	ctx := context.Background()
	ps, _, l := dirPacks(t)
	u := uploadOf(map[string]string{"profiles/can.yaml": "name: can\n", "params/LM1117.textproto": "mpn: \"LM1117\"\n"})
	in := &webapi.Pack{Config: &webapi.AnalysisConfig{ProfileUris: []string{"profiles"}, ParamUris: []string{"params"}}, Files: u.files}
	p, err := ps.CreatePack(ctx, "house", in, u.source)
	if err != nil {
		t.Fatal(err)
	}
	if p.GetName() != "packs/house" || p.GetTitle() != "house" || !packURI.MatchString(p.GetUri()) || !strings.HasPrefix(p.GetHash(), "sha256:") {
		t.Fatalf("pack = %v; want packs/house titled by its id, a view uri and a hash", p)
	}
	view := filepath.Join(ViewsRoot(l.uploads), filepath.FromSlash(strings.TrimPrefix(p.GetUri(), "mount://uploads/")))
	if b, err := os.ReadFile(filepath.Join(view, "profiles", "can.yaml")); err != nil || string(b) != "name: can\n" {
		t.Errorf("view's profiles/can.yaml = %q, %v; want the file sent", b, err)
	}
	got, err := ps.Pack(ctx, "house")
	if err != nil || got.GetHash() != p.GetHash() || got.GetUri() != p.GetUri() {
		t.Errorf("Pack read back = %v, %v; want what CreatePack returned", got, err)
	}
	if _, err := ps.CreatePack(ctx, "house", in, u.source); !errors.Is(err, service.ErrAlreadyExists) {
		t.Errorf("a second create = %v; want ErrAlreadyExists", err)
	}
}

// A pack's hash moves when its files' contents change and stays when nothing changes, and an old
// view stays for a snapshot still reading it.
func TestAPackHashFollowsItsFiles(t *testing.T) {
	ctx := context.Background()
	ps, _, _ := dirPacks(t)
	cfg := &webapi.AnalysisConfig{ProfileUris: []string{"profiles"}}
	v1 := uploadOf(map[string]string{"profiles/can.yaml": "name: can\n"})
	first, err := ps.CreatePack(ctx, "house", &webapi.Pack{Config: cfg, Files: v1.files}, v1.source)
	if err != nil {
		t.Fatal(err)
	}
	same, err := ps.UpdatePack(ctx, "house", func(p *webapi.Pack) error { return nil }, v1.source)
	if err != nil || same.GetHash() != first.GetHash() || same.GetUri() != first.GetUri() {
		t.Errorf("an update changing nothing = %v, %v; want the same hash and view", same, err)
	}
	v2 := uploadOf(map[string]string{"profiles/can.yaml": "name: can2\n"})
	next, err := ps.UpdatePack(ctx, "house", func(p *webapi.Pack) error { p.Files = v2.files; return nil }, v2.source)
	if err != nil {
		t.Fatal(err)
	}
	if next.GetHash() == first.GetHash() || next.GetUri() == first.GetUri() {
		t.Errorf("after the profile changed, hash %s and uri %s are unchanged; want both to move", next.GetHash(), next.GetUri())
	}
	conv := withConvention(t, ps, ctx)
	if conv.GetHash() == next.GetHash() {
		t.Errorf("a config change kept the hash %s; want it to move", conv.GetHash())
	}
}

// withConvention changes only the pack's convention, and returns the pack.
func withConvention(t *testing.T, ps *DirPacks, ctx context.Context) *webapi.Pack {
	t.Helper()
	p, err := ps.UpdatePack(ctx, "house", func(p *webapi.Pack) error {
		p.Config.Conventions = &configpb.NamingConvention{Name: "house"}
		return nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// A pack whose contents the server lacks is refused naming their hashes, and nothing is kept.
func TestAPackAsksForContentsItLacks(t *testing.T) {
	ctx := context.Background()
	ps, _, _ := dirPacks(t)
	u := uploadOf(map[string]string{"profiles/can.yaml": "name: can\n"})
	none := func(context.Context, string) ([]byte, bool) { return nil, false }
	_, err := ps.CreatePack(ctx, "house", &webapi.Pack{Files: u.files}, none)
	var missing *service.BlobsMissingError
	if !errors.As(err, &missing) || len(missing.Hashes) != 1 || missing.Hashes[0] != u.files[0].GetSha256() {
		t.Fatalf("create with no contents = %v; want BlobsMissing naming the one hash", err)
	}
	if _, err := ps.Pack(ctx, "house"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("after the refused create, Pack = %v; want ErrNotFound", err)
	}
}

// A store with no uploads keeps a pack naming mount:// URIs and refuses one with files.
func TestAPackStoreWithoutUploadsKeepsConfigOnly(t *testing.T) {
	ctx := context.Background()
	ps := NewPacks(recordstest.New(t), nil, "", "uploads")
	if p, err := ps.CreatePack(ctx, "shared", &webapi.Pack{Config: &webapi.AnalysisConfig{ProfileUris: []string{"mount://team/profiles"}}}, nil); err != nil || p.GetUri() != "" {
		t.Errorf("a config-only pack = %v, %v; want it kept, with no view", p, err)
	}
	u := uploadOf(map[string]string{"profiles/can.yaml": "name: can\n"})
	if _, err := ps.CreatePack(ctx, "house", &webapi.Pack{Files: u.files}, u.source); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("a pack with files = %v; want ErrFailedPrecondition naming --uploads", err)
	}
}

// Packs list by name, a delete removes one, and a second delete is NOT_FOUND.
func TestPacksListAndDelete(t *testing.T) {
	ctx := context.Background()
	ps, _, _ := dirPacks(t)
	for _, id := range []string{"b", "a"} {
		if _, err := ps.CreatePack(ctx, id, &webapi.Pack{}, nil); err != nil {
			t.Fatal(err)
		}
	}
	all, err := ps.Packs(ctx)
	if err != nil || len(all) != 2 || all[0].GetName() != "packs/a" || all[1].GetName() != "packs/b" {
		t.Fatalf("Packs = %v, %v; want packs/a then packs/b", all, err)
	}
	if err := ps.DeletePack(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if err := ps.DeletePack(ctx, "a"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("a second delete = %v; want ErrNotFound", err)
	}
	if _, err := ps.CreatePack(ctx, "../x", &webapi.Pack{}, nil); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("create ../x = %v; want ErrInvalidArgument", err)
	}
}

// The designs' sweep leaves a pack's view alone, and keeps its blobs when told their hashes.
func TestTheDesignSweepLeavesPackViews(t *testing.T) {
	ctx := context.Background()
	ps, ds, l := dirPacks(t)
	u := uploadOf(map[string]string{"profiles/can.yaml": "name: can\n"})
	p, err := ps.CreatePack(ctx, "house", &webapi.Pack{Config: &webapi.AnalysisConfig{ProfileUris: []string{"profiles"}}, Files: u.files}, u.source)
	if err != nil {
		t.Fatal(err)
	}
	keep, err := ps.Hashes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	views, blobs, err := ds.Sweep(keep, 0)
	if err != nil || views != 0 || blobs != 0 {
		t.Errorf("Sweep = %d views, %d blobs, %v; want nothing removed", views, blobs, err)
	}
	view := filepath.Join(ViewsRoot(l.uploads), filepath.FromSlash(strings.TrimPrefix(p.GetUri(), "mount://uploads/")), "profiles", "can.yaml")
	if _, err := os.Stat(view); err != nil {
		t.Errorf("after the sweep, the pack's view lost its file: %v", err)
	}
	if _, ok := ds.Blobs().Get(u.files[0].GetSha256()); !ok {
		t.Error("after the sweep, the pack's blob is gone")
	}
}

// A stored design keeps its intent, and its view's design.yaml declares it, so a read of the view
// checks against it (agni issue 1704).
func TestAStoredDesignKeepsItsIntent(t *testing.T) {
	ctx := context.Background()
	ds, l := dirStore(t)
	in := &configpb.DesignIntent{Nets: map[string]*configpb.NetIntent{"VCC_3V3": {Domain: "io"}}}
	if _, err := ds.CreateDesign(ctx, "gw", service.DesignRecord{Intent: in}); err != nil {
		t.Fatal(err)
	}
	d, err := ds.Design(ctx, "gw")
	if err != nil || d.GetConfig().GetIntent().GetNets()["VCC_3V3"].GetDomain() != "io" {
		t.Fatalf("a design with no artifacts = %v, %v; want its intent", d, err)
	}
	if _, err := put(ctx, ds, "gw", arts{"netlist": {"gw.edn": "(edif gw)"}}); err != nil {
		t.Fatal(err)
	}
	d, err = ds.Design(ctx, "gw")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(viewDir(l, d), DesignDescriptor))
	if err != nil {
		t.Fatal(err)
	}
	_, parsed, _, err := ParseDesign(strings.NewReader(string(raw)))
	if err != nil || parsed.GetConfig().GetIntent().GetNets()["VCC_3V3"].GetDomain() != "io" {
		t.Errorf("the view's design.yaml = %s, parsed %v, %v; want it to declare the intent", raw, parsed, err)
	}
	if _, err := ds.UpdateDesign(ctx, "gw", func(r *service.DesignRecord) { r.Title = "Gateway" }); err != nil {
		t.Fatal(err)
	}
	if d, _ := ds.Design(ctx, "gw"); d.GetConfig().GetIntent().GetNets()["VCC_3V3"] == nil {
		t.Error("an update of the title dropped the intent")
	}
}

// The pack sweep removes an earlier version's view and a deleted pack's views, and keeps the view a
// pack reads now and any view younger than the grace (agni issue 1746).
func TestThePackSweepKeepsOnlyCurrentViews(t *testing.T) {
	ctx := context.Background()
	ps, _, l := dirPacks(t)
	cfg := &webapi.AnalysisConfig{ProfileUris: []string{"profiles"}}
	v1 := uploadOf(map[string]string{"profiles/can.yaml": "name: can\n"})
	first, err := ps.CreatePack(ctx, "house", &webapi.Pack{Config: cfg, Files: v1.files}, v1.source)
	if err != nil {
		t.Fatal(err)
	}
	v2 := uploadOf(map[string]string{"profiles/can.yaml": "name: can2\n"})
	next, err := ps.UpdatePack(ctx, "house", func(p *webapi.Pack) error { p.Files = v2.files; return nil }, v2.source)
	if err != nil {
		t.Fatal(err)
	}
	gone, err := ps.CreatePack(ctx, "gone", &webapi.Pack{Config: cfg, Files: v1.files}, v1.source)
	if err != nil {
		t.Fatal(err)
	}
	if err := ps.DeletePack(ctx, "gone"); err != nil {
		t.Fatal(err)
	}
	viewOf := func(p *webapi.Pack) string {
		return filepath.Join(ViewsRoot(l.uploads), filepath.FromSlash(strings.TrimPrefix(p.GetUri(), "mount://uploads/")))
	}
	if n, err := ps.Sweep(ctx, time.Hour); err != nil || n != 0 {
		t.Errorf("a sweep inside the grace = %d, %v; want nothing removed", n, err)
	}
	n, err := ps.Sweep(ctx, 0)
	if err != nil || n != 2 {
		t.Fatalf("Sweep = %d, %v; want the old house view and the gone view removed", n, err)
	}
	for _, p := range []*webapi.Pack{first, gone} {
		if _, err := os.Stat(viewOf(p)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("view %s is still there: %v", p.GetUri(), err)
		}
	}
	if _, err := os.Stat(filepath.Dir(viewOf(gone))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the deleted pack's folder is still there: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(viewOf(next), "profiles", "can.yaml")); err != nil || string(b) != "name: can2\n" {
		t.Errorf("the current view's file = %q, %v; want it kept", b, err)
	}
}
