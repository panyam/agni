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
	"testing/fstest"

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

// dirStore is a DirStore over a fresh folder, served as mount "store" beside any other trees given.
func dirStore(t *testing.T, others ...Tree) (*DirStore, *FSStore, string) {
	t.Helper()
	root := t.TempDir()
	read := NewFSStore(append([]Tree{{Mount: "store", FS: os.DirFS(root)}}, others...)...)
	ds, err := NewDirStore(root, read)
	if err != nil {
		t.Fatal(err)
	}
	return ds, read, root
}

func mustProject(t *testing.T, ds *DirStore, id string) {
	t.Helper()
	u := uploadOf(map[string]string{"project.yaml": "title: " + id + "\n"})
	if _, err := ds.CreateProject(context.Background(), id, u.files, u.source); err != nil {
		t.Fatalf("CreateProject %s: %v", id, err)
	}
}

func TestADirStoreProjectAndDesignReadBack(t *testing.T) {
	ctx := context.Background()
	ds, read, root := dirStore(t)
	mustProject(t, ds, "b200")
	u := uploadOf(map[string]string{
		"design.yaml":   "title: Memory unit\nentry: mem.edn\ncompanions: [mem.kicad_pcb]\n",
		"mem.edn":       "(edif mem)",
		"mem.kicad_pcb": "(kicad_pcb)",
	})
	d, err := ds.CreateDesign(ctx, "projects/b200", "memory_unit", u.files, u.source)
	if err != nil {
		t.Fatal(err)
	}
	if d.GetName() != "projects/b200/designs/memory_unit" || d.GetEntryUri() != "mount://store/b200/designs/memory_unit/mem.edn" {
		t.Errorf("design = %s at %s, want projects/b200/designs/memory_unit at mount://store/b200/designs/memory_unit/mem.edn", d.GetName(), d.GetEntryUri())
	}
	ps, err := read.Projects(ctx)
	if err != nil || len(ps) != 1 || ps[0].GetName() != "projects/b200" || ps[0].GetTitle() != "b200" {
		t.Fatalf("projects = %v, %v; want projects/b200 titled b200", ps, err)
	}
	ds2, err := read.Designs(ctx, "projects/b200")
	if err != nil || len(ds2) != 1 || ds2[0].GetTitle() != "Memory unit" || len(ds2[0].GetCompanionUris()) != 1 {
		t.Fatalf("designs = %v, %v; want memory_unit with one companion", ds2, err)
	}
	got, err := os.ReadFile(filepath.Join(root, "b200", "designs", "memory_unit", "design.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"name: memory_unit", "extends: ../../project.yaml"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("stored design.yaml lacks %q:\n%s", want, got)
		}
	}
}

func TestADirStoreRefusesABadIdOrPath(t *testing.T) {
	ctx := context.Background()
	ds, _, _ := dirStore(t)
	for _, tc := range []struct {
		id    string
		files map[string]string
	}{
		{"B200", map[string]string{"project.yaml": "title: x\n"}},
		{"b200", map[string]string{"project.yaml": "name: other\n"}},
		{"b200", map[string]string{"notes.md": "no descriptor"}},
		{"b200", map[string]string{"project.yaml": "title: x\n", "../escape.yaml": "x"}},
		{"b200", map[string]string{"project.yaml": "title: x\n", ".hidden/x": "x"}},
		{"b200", map[string]string{"project.yaml": "title: x\n", "designs/d/design.yaml": "entry: a\n"}},
		{"b200", map[string]string{"project.yaml": "entry: a.edn\n"}},
	} {
		u := uploadOf(tc.files)
		if _, err := ds.CreateProject(ctx, tc.id, u.files, u.source); !errors.Is(err, service.ErrInvalidArgument) {
			t.Errorf("CreateProject(%q, %v) = %v, want ErrInvalidArgument", tc.id, tc.files, err)
		}
	}
	mustProject(t, ds, "b200")
	u := uploadOf(map[string]string{"project.yaml": "title: again\n"})
	if _, err := ds.CreateProject(ctx, "b200", u.files, u.source); !errors.Is(err, service.ErrAlreadyExists) {
		t.Errorf("second CreateProject = %v, want ErrAlreadyExists", err)
	}
	if _, err := ds.CreateDesign(ctx, "projects/none", "d", u.files, u.source); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("CreateDesign under a missing project = %v, want ErrNotFound", err)
	}
}

func TestADirStoreAsksForContentsItLacks(t *testing.T) {
	ctx := context.Background()
	ds, _, _ := dirStore(t)
	u := uploadOf(map[string]string{"project.yaml": "title: x\n", "intent.yaml": "modules: []\n"})
	held := upload{files: u.files, blobs: map[string][]byte{u.files[1].GetSha256(): u.blobs[u.files[1].GetSha256()]}}
	_, err := ds.CreateProject(ctx, "b200", held.files, held.source)
	var bm *service.BlobsMissingError
	if !errors.As(err, &bm) || len(bm.Hashes) != 1 || bm.Hashes[0] != u.files[0].GetSha256() {
		t.Fatalf("CreateProject with intent.yaml's contents missing = %v, want a BlobsMissingError naming %s", err, u.files[0].GetSha256())
	}
	wrong := upload{files: u.files, blobs: map[string][]byte{u.files[0].GetSha256(): []byte("other"), u.files[1].GetSha256(): u.blobs[u.files[1].GetSha256()]}}
	if _, err := ds.CreateProject(ctx, "b200", wrong.files, wrong.source); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("CreateProject with contents that do not match their hash = %v, want ErrInvalidArgument", err)
	}
}

func TestReplacingADesignSwapsItsWholeFileSet(t *testing.T) {
	ctx := context.Background()
	ds, read, root := dirStore(t)
	mustProject(t, ds, "b200")
	first := uploadOf(map[string]string{"design.yaml": "entry: a.edn\ncompanions: [a.kicad_pcb]\n", "a.edn": "(edif a)", "a.kicad_pcb": "(kicad_pcb)"})
	if _, err := ds.CreateDesign(ctx, "projects/b200", "mul", first.files, first.source); err != nil {
		t.Fatal(err)
	}
	next := uploadOf(map[string]string{"design.yaml": "entry: b.edn\n", "b.edn": "(edif b)"})
	d, err := ds.ReplaceDesign(ctx, "projects/b200/designs/mul", next.files, next.source)
	if err != nil {
		t.Fatal(err)
	}
	if d.GetEntryUri() != "mount://store/b200/designs/mul/b.edn" || len(d.GetCompanionUris()) != 0 {
		t.Errorf("replaced design = entry %s, companions %v; want b.edn and none", d.GetEntryUri(), d.GetCompanionUris())
	}
	entries, err := os.ReadDir(filepath.Join(root, "b200", "designs", "mul"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if !slices.Equal(names, []string{"b.edn", "design.yaml"}) {
		t.Errorf("design folder holds %v, want only b.edn and design.yaml", names)
	}
	if got, err := read.Design(ctx, "projects/b200/designs/mul"); err != nil || got.GetEntryUri() != d.GetEntryUri() {
		t.Errorf("read after replace = %v, %v; want entry %s", got, err, d.GetEntryUri())
	}
	if top, _ := os.ReadDir(root); len(top) != 1 {
		t.Errorf("store root holds %d entries after a replace, want only b200 (no temp or trash left)", len(top))
	}
}

func TestReplacingAProjectKeepsItsDesigns(t *testing.T) {
	ctx := context.Background()
	ds, read, _ := dirStore(t)
	mustProject(t, ds, "b200")
	u := uploadOf(map[string]string{"design.yaml": "entry: a.edn\n", "a.edn": "(edif a)"})
	if _, err := ds.CreateDesign(ctx, "projects/b200", "mul", u.files, u.source); err != nil {
		t.Fatal(err)
	}
	p := uploadOf(map[string]string{"project.yaml": "title: B200 rev 2\n"})
	got, err := ds.ReplaceProject(ctx, "projects/b200", p.files, p.source)
	if err != nil || got.GetTitle() != "B200 rev 2" {
		t.Fatalf("ReplaceProject = %v, %v; want the new title", got, err)
	}
	if designs, err := read.Designs(ctx, "projects/b200"); err != nil || len(designs) != 1 {
		t.Errorf("designs after replacing the project = %v, %v; want mul still there", designs, err)
	}
}

func TestDeletingAProjectThatHoldsDesignsNeedsCascade(t *testing.T) {
	ctx := context.Background()
	ds, read, _ := dirStore(t)
	mustProject(t, ds, "b200")
	u := uploadOf(map[string]string{"design.yaml": "entry: a.edn\n", "a.edn": "(edif a)"})
	if _, err := ds.CreateDesign(ctx, "projects/b200", "mul", u.files, u.source); err != nil {
		t.Fatal(err)
	}
	if err := ds.DeleteProject(ctx, "projects/b200", false); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("DeleteProject without cascade = %v, want ErrFailedPrecondition", err)
	}
	if err := ds.DeleteDesign(ctx, "projects/b200/designs/mul"); err != nil {
		t.Fatal(err)
	}
	if err := ds.DeleteDesign(ctx, "projects/b200/designs/mul"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("second DeleteDesign = %v, want ErrNotFound", err)
	}
	if _, err := ds.CreateDesign(ctx, "projects/b200", "mul", u.files, u.source); err != nil {
		t.Fatal(err)
	}
	if err := ds.DeleteProject(ctx, "projects/b200", true); err != nil {
		t.Fatalf("DeleteProject with cascade = %v", err)
	}
	if _, err := read.Project(ctx, "projects/b200"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("read after delete = %v, want ErrNotFound", err)
	}
}

func TestAMountedProjectIsReadOnly(t *testing.T) {
	ctx := context.Background()
	mounted := fstest.MapFS{
		"project.yaml":          {Data: []byte("name: tut\n")},
		"designs/g/design.yaml": {Data: []byte("name: g\nextends: ../../project.yaml\nentry: g.edn\n")},
		"designs/g/g.edn":       {Data: []byte("(edif g)")},
	}
	ds, _, _ := dirStore(t, Tree{Mount: "tut", FS: mounted})
	p := uploadOf(map[string]string{"project.yaml": "title: x\n"})
	if _, err := ds.ReplaceProject(ctx, "projects/tut", p.files, p.source); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("ReplaceProject on a mounted project = %v, want ErrFailedPrecondition", err)
	}
	if err := ds.DeleteProject(ctx, "projects/tut", true); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("DeleteProject on a mounted project = %v, want ErrFailedPrecondition", err)
	}
	d := uploadOf(map[string]string{"design.yaml": "entry: a.edn\n", "a.edn": "(edif a)"})
	if _, err := ds.CreateDesign(ctx, "projects/tut", "new", d.files, d.source); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("CreateDesign under a mounted project = %v, want ErrFailedPrecondition", err)
	}
	if err := ds.DeleteDesign(ctx, "projects/tut/designs/g"); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("DeleteDesign on a mounted design = %v, want ErrFailedPrecondition", err)
	}
	if _, err := ds.CreateProject(ctx, "tut", p.files, p.source); !errors.Is(err, service.ErrAlreadyExists) {
		t.Errorf("CreateProject over a mounted project's id = %v, want ErrAlreadyExists", err)
	}
}
