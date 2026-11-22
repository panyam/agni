package projects

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/records"
	"github.com/panyam/agni/internal/records/recordstest"
	"github.com/panyam/agni/service"
	"github.com/panyam/goapplib/storage/mounts"
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

// layout is where a test store keeps its records and its uploads. recs is the records store over
// records, a folder or sqlite as recordstest.BackendEnv picks.
type layout struct {
	records, uploads string
	recs             records.Store
}

// views is the folder the uploads mount serves.
func (l layout) views() string { return filepath.Join(l.uploads, "designs") }

// dirStore is a DirStore over fresh folders, its views served as mount "uploads".
func dirStore(t *testing.T) (*DirStore, layout) {
	t.Helper()
	l := layout{records: t.TempDir(), uploads: t.TempDir()}
	l.recs = recordstest.At(t, l.records)
	ds, err := NewStore(l.recs, l.uploads, "uploads")
	if err != nil {
		t.Fatal(err)
	}
	return ds, l
}

// revURI matches a stored design's base URI, its view of one revision.
var revURI = regexp.MustCompile(`^mount://uploads/[a-z0-9._-]+/[0-9a-f]{12}$`)

// inView is a file's URI in a design's current view, checking the view's URI first.
func inView(t *testing.T, d *webapi.Design, rel string) string {
	t.Helper()
	if !revURI.MatchString(d.GetUri()) {
		t.Fatalf("design uri = %q, want mount://uploads/<id>/<rev>", d.GetUri())
	}
	return d.GetUri() + "/" + rel
}

// viewDir is the folder of a design's current view on disk.
func viewDir(l layout, d *webapi.Design) string {
	return filepath.Join(l.uploads, "designs", filepath.FromSlash(strings.TrimPrefix(d.GetUri(), "mount://uploads/")))
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
	ds, l := dirStore(t)
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
	if d.GetEntryUri() != inView(t, d, "netlist/mem.edn") || !slices.Equal(d.GetCompanionUris(), []string{inView(t, d, "board/mem.kicad_pcb")}) {
		t.Errorf("design = entry %s, companions %v; want the netlist as entry and the board as its companion", d.GetEntryUri(), d.GetCompanionUris())
	}
	if d.GetTitle() != "Memory unit" || d.GetSeries() != "mem" || !slices.Equal(d.GetArtifacts(), []string{"designs/memory_unit/artifacts/board", "designs/memory_unit/artifacts/netlist"}) {
		t.Errorf("design = title %q, series %q, artifacts %v; want the record kept and both artifacts named", d.GetTitle(), d.GetSeries(), d.GetArtifacts())
	}
	desc, err := os.ReadFile(filepath.Join(viewDir(l, d), "design.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(desc), "name: memory_unit") || strings.Contains(string(desc), "extends") {
		t.Errorf("the view's design.yaml should name memory_unit and extend nothing:\n%s", desc)
	}
	// The view reads as a plain design, the way the engine and the viewer's mount paths open it.
	read := NewFSStore(Tree{Mount: "uploads", FS: os.DirFS(l.views())})
	rd, rp, err := read.ResolveDesign(ctx, artifactURI(t, d.GetEntryUri()))
	if err != nil || rd == nil || rp != nil || rd.GetTitle() != "Memory unit" {
		t.Errorf("ResolveDesign on the view = %v, %v, %v; want the design and no project", rd, rp, err)
	}
}

func TestTheStoreHoldsRecordsAndTheUploadsHoldFiles(t *testing.T) {
	ds, l := dirStore(t)
	const board = "(kicad_pcb board-bytes)"
	mustDesign(t, ds, "mul", arts{"netlist": {"a.edn": "(edif a)"}, "board": {"a.kicad_pcb": board}})
	if names := recordNames(t, l); !slices.Equal(names, []string{"mul.yaml"}) {
		t.Errorf("records hold %v, want only mul.yaml", names)
	}
	rec, err := l.recs.Get(context.Background(), "mul.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(rec), "board-bytes") || !strings.Contains(string(rec), hashOf([]byte(board))) {
		t.Errorf("the record should name the board by hash and hold none of its bytes:\n%s", rec)
	}
	b, ok := ds.Blobs().Get(hashOf([]byte(board)))
	if !ok || string(b) != board {
		t.Errorf("blob for the board = %q, %v; want its bytes", b, ok)
	}
}

func TestAStoreWithoutUploadsKeepsRecordsOnly(t *testing.T) {
	ctx := context.Background()
	ds, err := NewStore(recordstest.New(t), "", "uploads")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ds.CreateDesign(ctx, "mul", service.DesignRecord{Title: "Multiplier"}); err != nil {
		t.Fatalf("CreateDesign with no uploads = %v, want the record written", err)
	}
	if _, err := put(ctx, ds, "mul", arts{"netlist": {"a.edn": "(edif a)"}}); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("a write of files with no uploads = %v, want ErrFailedPrecondition", err)
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
	if d, err := ds.Design(ctx, "bb"); err != nil || d.GetEntryUri() != inView(t, d, "schematic/bb.kicad_sch") {
		t.Errorf("design = %v, %v; want the root sheet as entry", d, err)
	}
}

func TestADesignNoReaderOpensIsADraft(t *testing.T) {
	ctx := context.Background()
	ds, l := dirStore(t)
	d, err := ds.CreateDesign(ctx, "mul", service.DesignRecord{Title: "Multiplier"})
	if err != nil {
		t.Fatal(err)
	}
	if d.GetEntryUri() != "" || d.GetTitle() != "Multiplier" || d.GetUri() != "mount://uploads/mul" {
		t.Errorf("new design = entry %q, title %q, uri %q; want no entry, its title, and its folder", d.GetEntryUri(), d.GetTitle(), d.GetUri())
	}
	got, err := put(ctx, ds, "mul", arts{"bom": {"bom.csv": "ref,mpn\n"}})
	if err != nil || len(got) != 1 || got[0].GetType() != webapi.ArtifactType_ARTIFACT_TYPE_OTHER || got[0].GetEntry() != "" {
		t.Fatalf("put bom = %v, %v; want one artifact of type other with no entry", got, err)
	}
	if _, err := os.Stat(filepath.Join(l.views(), "mul")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a design no reader opens has a view (%v); want none, since no reader opens it", err)
	}
	if all, err := ds.Designs(ctx); err != nil || len(all) != 1 || all[0].GetName() != "designs/mul" {
		t.Errorf("designs = %v, %v; want the draft listed", all, err)
	}
	// A project walk over the uploads mount parses every design.yaml, and a draft must not break it.
	read := NewFSStore(Tree{Mount: "uploads", FS: os.DirFS(l.views())})
	if _, err := read.Projects(ctx); err != nil {
		t.Errorf("Projects over a store holding a draft = %v", err)
	}
}

func TestAStoreRefusesADesignItCouldReadTwoWays(t *testing.T) {
	ctx := context.Background()
	ds, l := dirStore(t)
	mustDesign(t, ds, "mul", arts{"netlist": {"mul.edn": "(edif mul)"}})
	before, _ := l.recs.Get(ctx, "mul.yaml")
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
	after, _ := l.recs.Get(ctx, "mul.yaml")
	if len(before) == 0 || string(before) != string(after) {
		t.Errorf("a refused write changed the record:\n%s\nto\n%s", before, after)
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
	ds, l := dirStore(t)
	old := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	ds.now = func() time.Time { return old }
	mustDesign(t, ds, "mul", arts{"netlist": {"a.edn": "(edif a)"}, "board": {"a.kicad_pcb": "(kicad_pcb)"}})
	first, err := ds.Design(ctx, "mul")
	if err != nil {
		t.Fatal(err)
	}
	ds.now = func() time.Time { return old.Add(time.Hour) }
	if _, err := put(ctx, ds, "mul", arts{"netlist": {"b.edn": "(edif b)"}}); err != nil {
		t.Fatal(err)
	}
	d, err := ds.Design(ctx, "mul")
	if err != nil {
		t.Fatal(err)
	}
	if d.GetEntryUri() != inView(t, d, "netlist/b.edn") || !slices.Equal(d.GetCompanionUris(), []string{inView(t, d, "board/a.kicad_pcb")}) {
		t.Errorf("design = entry %s, companions %v; want netlist/b.edn and the board kept", d.GetEntryUri(), d.GetCompanionUris())
	}
	if names := dirNames(t, filepath.Join(viewDir(l, d), "netlist")); !slices.Equal(names, []string{"b.edn"}) {
		t.Errorf("netlist holds %v, want only b.edn", names)
	}
	all, err := ds.Artifacts(ctx, "mul")
	if err != nil || len(all) != 2 || !all[0].GetUpdateTime().AsTime().Equal(old) || !all[1].GetUpdateTime().AsTime().Equal(old.Add(time.Hour)) {
		t.Errorf("artifacts = %v, %v; want the untouched board's update_time kept at %v and the netlist's moved", all, err, old)
	}
	// The old revision's view is never changed, so a snapshot read from it keeps answering, and the
	// replaced file's bytes stay until the sweep.
	if first.GetUri() == d.GetUri() {
		t.Fatalf("a replace kept the view %s, want a new revision", d.GetUri())
	}
	if names := dirNames(t, filepath.Join(viewDir(l, first), "netlist")); !slices.Equal(names, []string{"a.edn"}) {
		t.Errorf("the old view's netlist holds %v, want a.edn still", names)
	}
	if _, ok := ds.Blobs().Get(hashOf([]byte("(edif a)"))); !ok {
		t.Error("the replaced netlist's blob is gone before any sweep")
	}
	if revs := dirNames(t, filepath.Join(l.views(), "mul")); len(revs) != 2 {
		t.Errorf("the design's views are %v, want the two revisions and no temp folder", revs)
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
	if d.GetSeries() != "mul" || d.GetEntryUri() != inView(t, d, "netlist/a.edn") || len(d.GetArtifacts()) != 1 {
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
	if d, err := ds.Design(ctx, "mul"); err != nil || d.GetEntryUri() != inView(t, d, "board/a.kicad_pcb") || len(d.GetCompanionUris()) != 0 {
		t.Errorf("design after deleting its netlist = %v, %v; want the board as entry", d, err)
	}
	if err := ds.DeleteArtifact(ctx, "mul", "netlist"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("second DeleteArtifact = %v, want ErrNotFound", err)
	}
}

func TestDeletingADesignRemovesItsRecordAndTheSweepItsFiles(t *testing.T) {
	ctx := context.Background()
	ds, l := dirStore(t)
	mustDesign(t, ds, "mul", arts{"netlist": {"a.edn": "(edif a)"}})
	if err := ds.DeleteDesign(ctx, "mul"); err != nil {
		t.Fatal(err)
	}
	if err := ds.DeleteDesign(ctx, "mul"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("second DeleteDesign = %v, want ErrNotFound", err)
	}
	if top := recordNames(t, l); len(top) != 0 {
		t.Errorf("records hold %v after the delete, want nothing", top)
	}
	if all, err := ds.Designs(ctx); err != nil || len(all) != 0 {
		t.Errorf("designs after the delete = %v, %v; want none", all, err)
	}
	views, blobs, err := ds.Sweep(nil, 0)
	if err != nil || views != 1 || blobs != 1 {
		t.Errorf("Sweep after the delete = %d views, %d blobs, %v; want the one view and the one blob removed", views, blobs, err)
	}
	if top := dirNames(t, l.views()); len(top) != 0 {
		t.Errorf("uploads mount holds %v after the sweep, want nothing", top)
	}
}

func TestTheSweepKeepsWhatIsNamedAndWhatIsNew(t *testing.T) {
	ctx := context.Background()
	ds, l := dirStore(t)
	mustDesign(t, ds, "mul", arts{"netlist": {"a.edn": "(edif a)"}})
	if _, err := put(ctx, ds, "mul", arts{"netlist": {"b.edn": "(edif b)"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := ds.Blobs().Put([]byte("kept by a snapshot")); err != nil {
		t.Fatal(err)
	}
	if views, blobs, err := ds.Sweep(nil, time.Hour); err != nil || views != 0 || blobs != 0 {
		t.Errorf("Sweep within the grace hour = %d views, %d blobs, %v; want nothing removed", views, blobs, err)
	}
	ago := time.Now().Add(-2 * time.Hour)
	if err := filepath.WalkDir(l.uploads, func(p string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Chtimes(p, ago, ago)
	}); err != nil {
		t.Fatal(err)
	}
	snap := hashOf([]byte("kept by a snapshot"))
	views, blobs, err := ds.Sweep(map[string]bool{snap: true}, time.Hour)
	if err != nil || views != 1 || blobs != 1 {
		t.Errorf("Sweep past the grace hour = %d views, %d blobs, %v; want the old view and a.edn's blob removed", views, blobs, err)
	}
	for h, want := range map[string]bool{hashOf([]byte("(edif a)")): false, hashOf([]byte("(edif b)")): true, snap: true} {
		if _, ok := ds.Blobs().Get(h); ok != want {
			t.Errorf("blob %s held = %v after the sweep, want %v", h, ok, want)
		}
	}
	d, err := ds.Design(ctx, "mul")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(viewDir(l, d), "netlist", "b.edn")); err != nil {
		t.Errorf("the current view lost its netlist in the sweep: %v", err)
	}
}

// recordNames lists what sits at the top of l's records. A folder lists every entry, dot files
// included, so a temp file left behind shows. sqlite lists its keys.
func recordNames(t *testing.T, l layout) []string {
	t.Helper()
	if recordstest.Folder() {
		return dirNames(t, l.records)
	}
	ents, err := l.recs.List(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		out = append(out, e.Name)
	}
	return out
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

func artifactURI(t *testing.T, s string) mounts.URI {
	t.Helper()
	u, err := mounts.ParseURI(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// Two servers share one store and add a different artifact to one design at the same moment. Both
// artifacts stay, since each write goes through records.Update and the loser reads the winner's
// record again (agni issue 1823). Each server is a DirStore of its own over the same records and
// uploads, so the in-process mutex doesn't serialize them.
func TestTwoServersWritingOneDesignLoseNeitherArtifact(t *testing.T) {
	ctx := context.Background()
	l := layout{records: t.TempDir(), uploads: t.TempDir()}
	a, err := NewStore(recordstest.At(t, l.records), l.uploads, "uploads")
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewStore(recordstest.At(t, l.records), l.uploads, "uploads")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateDesign(ctx, "mul", service.DesignRecord{}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.CreateDesign(ctx, "mul", service.DesignRecord{}); !errors.Is(err, service.ErrAlreadyExists) {
		t.Fatalf("the second server's create of mul = %v, want ErrAlreadyExists", err)
	}
	const rounds = 6
	for i := range rounds {
		var wg sync.WaitGroup
		errs := make([]error, 2)
		for j, ds := range []*DirStore{a, b} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				id := fmt.Sprintf("notes-%d-%d", i, j)
				_, errs[j] = put(ctx, ds, "mul", arts{id: {id + ".txt": id}})
			}()
		}
		wg.Wait()
		for _, err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	all, err := a.Artifacts(ctx, "mul")
	if err != nil || len(all) != 2*rounds {
		t.Fatalf("after %d rounds of two racing writes, mul holds %d artifacts, %v; want %d", rounds, len(all), err, 2*rounds)
	}
}
