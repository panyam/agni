package projects

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// upload is a file set as a client sends it: a manifest, and the contents it names.
type upload struct {
	files []*webapi.SnapshotFile
	blobs map[string][]byte
}

func uploadOf(files map[string]string) upload {
	u := upload{blobs: map[string][]byte{}}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	for _, p := range paths {
		b := []byte(files[p])
		sum := sha256.Sum256(b)
		h := "sha256:" + hex.EncodeToString(sum[:])
		u.files = append(u.files, &webapi.SnapshotFile{Path: p, Sha256: h, Size: int64(len(b))})
		u.blobs[h] = b
	}
	return u
}

func (u upload) source(_ context.Context, h string) ([]byte, bool) {
	b, ok := u.blobs[h]
	return b, ok
}

// dirStore is a DirStore over a fresh folder, served as mount "store".
func dirStore(t *testing.T) (*DirStore, string) {
	t.Helper()
	root := t.TempDir()
	ds, err := NewDirStore(root, "store")
	if err != nil {
		t.Fatal(err)
	}
	return ds, root
}

// arts maps each artifact id to its files, path to contents.
type arts map[string]map[string]string

// put writes artifacts to a design in one batch, creating any it lacks.
func put(ctx context.Context, ds service.DesignStore, id string, as arts) ([]*webapi.Artifact, error) {
	ids := make([]string, 0, len(as))
	for a := range as {
		ids = append(ids, a)
	}
	slices.Sort(ids)
	var writes []service.ArtifactWrite
	blobs := map[string][]byte{}
	for _, a := range ids {
		u := uploadOf(as[a])
		for h, b := range u.blobs {
			blobs[h] = b
		}
		writes = append(writes, service.ArtifactWrite{ID: a, Files: u.files, AllowMissing: true})
	}
	return ds.WriteArtifacts(ctx, id, writes, func(_ context.Context, h string) ([]byte, bool) {
		b, ok := blobs[h]
		return b, ok
	})
}

func mustDesign(t *testing.T, ds service.DesignStore, id string, as arts) {
	t.Helper()
	ctx := context.Background()
	if _, err := ds.CreateDesign(ctx, id, service.DesignRecord{}); err != nil {
		t.Fatalf("CreateDesign %s: %v", id, err)
	}
	if _, err := put(ctx, ds, id, as); err != nil {
		t.Fatalf("WriteArtifacts %s: %v", id, err)
	}
}

func TestADesignReadsBackFromItsArtifacts(t *testing.T) {
	ctx := context.Background()
	ds, root := dirStore(t)
	if _, err := ds.CreateDesign(ctx, "memory_unit", service.DesignRecord{Title: "Memory unit", Series: "mem"}); err != nil {
		t.Fatal(err)
	}
	got, err := put(ctx, ds, "memory_unit", arts{"netlist": {"mem.edn": "(edif mem)"}, "board": {"mem.kicad_pcb": "(kicad_pcb)"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].GetType() != webapi.ArtifactType_ARTIFACT_TYPE_BOARD || got[0].GetEntry() != "mem.kicad_pcb" ||
		got[1].GetType() != webapi.ArtifactType_ARTIFACT_TYPE_NETLIST || got[1].GetEntry() != "mem.edn" {
		t.Errorf("written artifacts = %v; want board with entry mem.kicad_pcb, then netlist with entry mem.edn", got)
	}
	d, err := ds.Design(ctx, "memory_unit")
	if err != nil {
		t.Fatal(err)
	}
	if d.GetEntryUri() != "mount://store/memory_unit/netlist/mem.edn" || !slices.Equal(d.GetCompanionUris(), []string{"mount://store/memory_unit/board/mem.kicad_pcb"}) {
		t.Errorf("design = entry %s, companions %v; want the netlist as entry and the board as its companion", d.GetEntryUri(), d.GetCompanionUris())
	}
	if d.GetTitle() != "Memory unit" || d.GetSeries() != "mem" || !slices.Equal(d.GetArtifacts(), []string{"designs/memory_unit/artifacts/board", "designs/memory_unit/artifacts/netlist"}) {
		t.Errorf("design = title %q, series %q, artifacts %v; want the record kept and both artifacts named", d.GetTitle(), d.GetSeries(), d.GetArtifacts())
	}
	desc, err := os.ReadFile(filepath.Join(root, "memory_unit", "design.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(desc), "name: memory_unit") || strings.Contains(string(desc), "extends") {
		t.Errorf("stored design.yaml should name memory_unit and extend nothing:\n%s", desc)
	}
	// The folder reads as a plain design, the way the CLI and the viewer's mount paths open it.
	read := NewFSStore(Tree{Mount: "store", FS: os.DirFS(root)})
	rd, rp, err := read.ResolveDesign(ctx, artifactURI(t, "mount://store/memory_unit/netlist/mem.edn"))
	if err != nil || rd == nil || rp != nil || rd.GetTitle() != "Memory unit" {
		t.Errorf("ResolveDesign on the stored folder = %v, %v, %v; want the design and no project", rd, rp, err)
	}
}

func TestAKiCadSchematicAndItsSheetsAreOneArtifact(t *testing.T) {
	ctx := context.Background()
	ds, _ := dirStore(t)
	mustDesign(t, ds, "bb", arts{"schematic": {
		"bb.kicad_sch":        `(kicad_sch (sheet (property "Sheetname" "Power") (property "Sheetfile" "sub/power.kicad_sch")))`,
		"sub/power.kicad_sch": "(kicad_sch)",
	}})
	all, err := ds.Artifacts(ctx, "bb")
	if err != nil || len(all) != 1 {
		t.Fatalf("artifacts = %v, %v; want one", all, err)
	}
	a := all[0]
	if a.GetType() != webapi.ArtifactType_ARTIFACT_TYPE_SCHEMATIC || a.GetEntry() != "bb.kicad_sch" || len(a.GetFiles()) != 2 || a.GetFiles()[1].GetPath() != "sub/power.kicad_sch" {
		t.Errorf("artifact = %v; want a schematic entered at bb.kicad_sch holding both sheets", a)
	}
	if d, err := ds.Design(ctx, "bb"); err != nil || d.GetEntryUri() != "mount://store/bb/schematic/bb.kicad_sch" {
		t.Errorf("design = %v, %v; want the root sheet as entry", d, err)
	}
}

func TestADesignNoReaderOpensIsADraft(t *testing.T) {
	ctx := context.Background()
	ds, root := dirStore(t)
	d, err := ds.CreateDesign(ctx, "mul", service.DesignRecord{Title: "Multiplier"})
	if err != nil {
		t.Fatal(err)
	}
	if d.GetEntryUri() != "" || d.GetTitle() != "Multiplier" || d.GetUri() != "mount://store/mul" {
		t.Errorf("new design = entry %q, title %q, uri %q; want no entry, its title, and its folder", d.GetEntryUri(), d.GetTitle(), d.GetUri())
	}
	got, err := put(ctx, ds, "mul", arts{"bom": {"bom.csv": "ref,mpn\n"}})
	if err != nil || len(got) != 1 || got[0].GetType() != webapi.ArtifactType_ARTIFACT_TYPE_OTHER || got[0].GetEntry() != "" {
		t.Fatalf("put bom = %v, %v; want one artifact of type other with no entry", got, err)
	}
	if _, err := os.Stat(filepath.Join(root, "mul", "design.yaml")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a design no reader opens has a design.yaml (%v); want only design.draft.yaml", err)
	}
	if all, err := ds.Designs(ctx); err != nil || len(all) != 1 || all[0].GetName() != "designs/mul" {
		t.Errorf("designs = %v, %v; want the draft listed", all, err)
	}
	// A project walk over the store parses every design.yaml, and a draft must not break it.
	read := NewFSStore(Tree{Mount: "store", FS: os.DirFS(root)})
	if _, err := read.Projects(ctx); err != nil {
		t.Errorf("Projects over a store holding a draft = %v", err)
	}
}

func TestAStoreRefusesADesignItCouldReadTwoWays(t *testing.T) {
	ctx := context.Background()
	ds, root := dirStore(t)
	mustDesign(t, ds, "mul", arts{"netlist": {"mul.edn": "(edif mul)"}})
	before, _ := os.ReadFile(filepath.Join(root, "mul", "design.yaml"))
	for _, tc := range []struct {
		name string
		as   arts
		want error
	}{
		{"a second netlist", arts{"other": {"other.edn": "(edif other)"}}, service.ErrFailedPrecondition},
		{"two boards", arts{"sch": {"mul.kicad_sch": "(kicad_sch)", "mul.kicad_pcb": "(kicad_pcb)"}, "board": {"b.kicad_pcb": "(kicad_pcb)"}}, service.ErrFailedPrecondition},
		{"one artifact making two designs", arts{"two": {"alpha.edn": "(edif alpha)", "beta.edn": "(edif beta)"}}, service.ErrInvalidArgument},
	} {
		if _, err := put(ctx, ds, "mul", tc.as); !errors.Is(err, tc.want) {
			t.Errorf("%s = %v, want %v", tc.name, err, tc.want)
		}
	}
	after, _ := os.ReadFile(filepath.Join(root, "mul", "design.yaml"))
	if string(before) != string(after) {
		t.Errorf("a refused write changed design.yaml:\n%s\nto\n%s", before, after)
	}
	if all, err := ds.Artifacts(ctx, "mul"); err != nil || len(all) != 1 {
		t.Errorf("artifacts after refused writes = %v, %v; want only netlist", all, err)
	}
}

func TestADirStoreRefusesABadIdOrPath(t *testing.T) {
	ctx := context.Background()
	ds, _ := dirStore(t)
	if _, err := ds.CreateDesign(ctx, "Mul", service.DesignRecord{}); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("CreateDesign(Mul) = %v, want ErrInvalidArgument", err)
	}
	mustDesign(t, ds, "mul", arts{"netlist": {"a.edn": "(edif a)"}})
	for _, as := range []arts{
		{"Bad": {"a.edn": "x"}},
		{"design.yaml": {"a.edn": "x"}},
		{"netlist": {"../escape.edn": "x"}},
		{"netlist": {".hidden/x": "x"}},
		{"netlist": {"design.yaml": "entry: a.edn\n", "a.edn": "x"}},
		{"netlist": {"sub/project.yaml": "name: p\n", "a.edn": "x"}},
	} {
		if _, err := put(ctx, ds, "mul", as); !errors.Is(err, service.ErrInvalidArgument) {
			t.Errorf("put %v = %v, want ErrInvalidArgument", as, err)
		}
	}
	if _, err := ds.CreateDesign(ctx, "mul", service.DesignRecord{}); !errors.Is(err, service.ErrAlreadyExists) {
		t.Errorf("second CreateDesign = %v, want ErrAlreadyExists", err)
	}
	u := uploadOf(map[string]string{"b.edn": "(edif b)"})
	if _, err := ds.WriteArtifacts(ctx, "mul", []service.ArtifactWrite{{ID: "board", Files: u.files}}, u.source); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("a write to a missing artifact without allow_missing = %v, want ErrNotFound", err)
	}
	if _, err := put(ctx, ds, "none", arts{"netlist": {"a.edn": "x"}}); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("a write to a missing design = %v, want ErrNotFound", err)
	}
}

func TestADirStoreAsksForContentsItLacks(t *testing.T) {
	ctx := context.Background()
	ds, _ := dirStore(t)
	if _, err := ds.CreateDesign(ctx, "mul", service.DesignRecord{}); err != nil {
		t.Fatal(err)
	}
	u := uploadOf(map[string]string{"a.edn": "(edif a)", "a.kicad_pcb": "(kicad_pcb)"})
	pcb := u.files[1].GetSha256()
	held := upload{files: u.files, blobs: map[string][]byte{pcb: u.blobs[pcb]}}
	_, err := ds.WriteArtifacts(ctx, "mul", []service.ArtifactWrite{{ID: "netlist", Files: held.files, AllowMissing: true}}, held.source)
	var bm *service.BlobsMissingError
	if !errors.As(err, &bm) || len(bm.Hashes) != 1 || bm.Hashes[0] != u.files[0].GetSha256() {
		t.Fatalf("a write with a.edn's contents missing = %v, want a BlobsMissingError naming %s", err, u.files[0].GetSha256())
	}
	wrong := upload{files: u.files, blobs: map[string][]byte{u.files[0].GetSha256(): []byte("other"), pcb: u.blobs[pcb]}}
	if _, err := ds.WriteArtifacts(ctx, "mul", []service.ArtifactWrite{{ID: "netlist", Files: wrong.files, AllowMissing: true}}, wrong.source); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("a write with contents that do not match their hash = %v, want ErrInvalidArgument", err)
	}
}

func TestReplacingAnArtifactSwapsItsFilesAndKeepsTheOthers(t *testing.T) {
	ctx := context.Background()
	ds, root := dirStore(t)
	mustDesign(t, ds, "mul", arts{"netlist": {"a.edn": "(edif a)"}, "board": {"a.kicad_pcb": "(kicad_pcb)"}})
	board := filepath.Join(root, "mul", "board", "a.kicad_pcb")
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(board, old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := put(ctx, ds, "mul", arts{"netlist": {"b.edn": "(edif b)"}}); err != nil {
		t.Fatal(err)
	}
	d, err := ds.Design(ctx, "mul")
	if err != nil {
		t.Fatal(err)
	}
	if d.GetEntryUri() != "mount://store/mul/netlist/b.edn" || !slices.Equal(d.GetCompanionUris(), []string{"mount://store/mul/board/a.kicad_pcb"}) {
		t.Errorf("design = entry %s, companions %v; want netlist/b.edn and the board kept", d.GetEntryUri(), d.GetCompanionUris())
	}
	if names := dirNames(t, filepath.Join(root, "mul", "netlist")); !slices.Equal(names, []string{"b.edn"}) {
		t.Errorf("netlist holds %v, want only b.edn", names)
	}
	all, err := ds.Artifacts(ctx, "mul")
	if err != nil || len(all) != 2 || !all[0].GetUpdateTime().AsTime().Equal(old) {
		t.Errorf("artifacts = %v, %v; want the untouched board's update_time kept at %v", all, err, old)
	}
	if top := dirNames(t, root); !slices.Equal(top, []string{"mul"}) {
		t.Errorf("store root holds %v after a replace, want only mul (no temp or trash left)", top)
	}
}

func TestUpdatingADesignKeepsItsArtifacts(t *testing.T) {
	ctx := context.Background()
	ds, _ := dirStore(t)
	mustDesign(t, ds, "mul", arts{"netlist": {"a.edn": "(edif a)"}})
	d, err := ds.UpdateDesign(ctx, "mul", func(r *service.DesignRecord) { r.Series = "mul" })
	if err != nil {
		t.Fatal(err)
	}
	if d.GetSeries() != "mul" || d.GetEntryUri() != "mount://store/mul/netlist/a.edn" || len(d.GetArtifacts()) != 1 {
		t.Errorf("updated design = series %q, entry %q, artifacts %v; want the series set and the netlist kept", d.GetSeries(), d.GetEntryUri(), d.GetArtifacts())
	}
}

func TestDeletingAnArtifactRecomposesTheDesign(t *testing.T) {
	ctx := context.Background()
	ds, _ := dirStore(t)
	mustDesign(t, ds, "mul", arts{"netlist": {"a.edn": "(edif a)"}, "board": {"a.kicad_pcb": "(kicad_pcb)"}})
	if err := ds.DeleteArtifact(ctx, "mul", "netlist"); err != nil {
		t.Fatal(err)
	}
	if d, err := ds.Design(ctx, "mul"); err != nil || d.GetEntryUri() != "mount://store/mul/board/a.kicad_pcb" || len(d.GetCompanionUris()) != 0 {
		t.Errorf("design after deleting its netlist = %v, %v; want the board as entry", d, err)
	}
	if err := ds.DeleteArtifact(ctx, "mul", "netlist"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("second DeleteArtifact = %v, want ErrNotFound", err)
	}
}

func TestDeletingADesignRemovesItsFolder(t *testing.T) {
	ctx := context.Background()
	ds, root := dirStore(t)
	mustDesign(t, ds, "mul", arts{"netlist": {"a.edn": "(edif a)"}})
	if err := ds.DeleteDesign(ctx, "mul"); err != nil {
		t.Fatal(err)
	}
	if err := ds.DeleteDesign(ctx, "mul"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("second DeleteDesign = %v, want ErrNotFound", err)
	}
	if top, _ := os.ReadDir(root); len(top) != 0 {
		t.Errorf("store root holds %d entries after the delete, want none", len(top))
	}
	if all, err := ds.Designs(ctx); err != nil || len(all) != 0 {
		t.Errorf("designs after the delete = %v, %v; want none", all, err)
	}
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func artifactURI(t *testing.T, s string) artifact.URI {
	t.Helper()
	u, err := artifact.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
