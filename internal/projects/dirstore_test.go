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

func mustDesign(t *testing.T, ds service.DesignStore, id string, files map[string]string) {
	t.Helper()
	u := uploadOf(files)
	if _, err := ds.CreateDesign(context.Background(), id, u.files, u.source); err != nil {
		t.Fatalf("CreateDesign %s: %v", id, err)
	}
}

func TestADirStoreDesignReadsBack(t *testing.T) {
	ctx := context.Background()
	ds, root := dirStore(t)
	u := uploadOf(map[string]string{
		"design.yaml":   "title: Memory unit\nentry: mem.edn\ncompanions: [mem.kicad_pcb]\n",
		"mem.edn":       "(edif mem)",
		"mem.kicad_pcb": "(kicad_pcb)",
	})
	d, err := ds.CreateDesign(ctx, "memory_unit", u.files, u.source)
	if err != nil {
		t.Fatal(err)
	}
	if d.GetName() != "designs/memory_unit" || d.GetEntryUri() != "mount://store/memory_unit/mem.edn" {
		t.Errorf("design = %s at %s, want designs/memory_unit at mount://store/memory_unit/mem.edn", d.GetName(), d.GetEntryUri())
	}
	all, err := ds.Designs(ctx)
	if err != nil || len(all) != 1 || all[0].GetTitle() != "Memory unit" || len(all[0].GetCompanionUris()) != 1 {
		t.Fatalf("designs = %v, %v; want memory_unit with one companion", all, err)
	}
	got, err := os.ReadFile(filepath.Join(root, "memory_unit", "design.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "name: memory_unit") || strings.Contains(string(got), "extends") {
		t.Errorf("stored design.yaml should name memory_unit and extend nothing:\n%s", got)
	}
	// The folder reads as a plain design, the way the CLI and the viewer's mount paths open it.
	read := NewFSStore(Tree{Mount: "store", FS: os.DirFS(root)})
	rd, rp, err := read.ResolveDesign(ctx, artifactURI(t, "mount://store/memory_unit/mem.edn"))
	if err != nil || rd == nil || rp != nil || rd.GetTitle() != "Memory unit" {
		t.Errorf("ResolveDesign on the stored folder = %v, %v, %v; want the design and no project", rd, rp, err)
	}
}

func TestADirStoreRefusesABadIdOrPath(t *testing.T) {
	ctx := context.Background()
	ds, _ := dirStore(t)
	ok := "entry: a.edn\n"
	for _, tc := range []struct {
		id    string
		files map[string]string
	}{
		{"Mul", map[string]string{"design.yaml": ok, "a.edn": "x"}},
		{"mul", map[string]string{"design.yaml": "name: other\n" + ok, "a.edn": "x"}},
		{"mul", map[string]string{"notes.md": "no descriptor"}},
		{"mul", map[string]string{"design.yaml": ok, "../escape.edn": "x"}},
		{"mul", map[string]string{"design.yaml": ok, ".hidden/x": "x"}},
		{"mul", map[string]string{"design.yaml": "extends: ../project.yaml\n" + ok, "a.edn": "x"}},
		{"mul", map[string]string{"design.yaml": "title: no entry\n"}},
	} {
		u := uploadOf(tc.files)
		if _, err := ds.CreateDesign(ctx, tc.id, u.files, u.source); !errors.Is(err, service.ErrInvalidArgument) {
			t.Errorf("CreateDesign(%q, %v) = %v, want ErrInvalidArgument", tc.id, tc.files, err)
		}
	}
	mustDesign(t, ds, "mul", map[string]string{"design.yaml": ok, "a.edn": "x"})
	u := uploadOf(map[string]string{"design.yaml": ok, "a.edn": "y"})
	if _, err := ds.CreateDesign(ctx, "mul", u.files, u.source); !errors.Is(err, service.ErrAlreadyExists) {
		t.Errorf("second CreateDesign = %v, want ErrAlreadyExists", err)
	}
	if _, err := ds.ReplaceDesign(ctx, "none", u.files, u.source); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("ReplaceDesign of a missing design = %v, want ErrNotFound", err)
	}
}

func TestADirStoreAsksForContentsItLacks(t *testing.T) {
	ctx := context.Background()
	ds, _ := dirStore(t)
	u := uploadOf(map[string]string{"a.edn": "(edif a)", "design.yaml": "entry: a.edn\n"})
	desc := u.files[1].GetSha256()
	held := upload{files: u.files, blobs: map[string][]byte{desc: u.blobs[desc]}}
	_, err := ds.CreateDesign(ctx, "mul", held.files, held.source)
	var bm *service.BlobsMissingError
	if !errors.As(err, &bm) || len(bm.Hashes) != 1 || bm.Hashes[0] != u.files[0].GetSha256() {
		t.Fatalf("CreateDesign with a.edn's contents missing = %v, want a BlobsMissingError naming %s", err, u.files[0].GetSha256())
	}
	wrong := upload{files: u.files, blobs: map[string][]byte{u.files[0].GetSha256(): []byte("other"), desc: u.blobs[desc]}}
	if _, err := ds.CreateDesign(ctx, "mul", wrong.files, wrong.source); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("CreateDesign with contents that do not match their hash = %v, want ErrInvalidArgument", err)
	}
}

func TestReplacingADesignSwapsItsWholeFileSet(t *testing.T) {
	ctx := context.Background()
	ds, root := dirStore(t)
	mustDesign(t, ds, "mul", map[string]string{"design.yaml": "entry: a.edn\ncompanions: [a.kicad_pcb]\n", "a.edn": "(edif a)", "a.kicad_pcb": "(kicad_pcb)"})
	next := uploadOf(map[string]string{"design.yaml": "entry: b.edn\n", "b.edn": "(edif b)"})
	d, err := ds.ReplaceDesign(ctx, "mul", next.files, next.source)
	if err != nil {
		t.Fatal(err)
	}
	if d.GetEntryUri() != "mount://store/mul/b.edn" || len(d.GetCompanionUris()) != 0 {
		t.Errorf("replaced design = entry %s, companions %v; want b.edn and none", d.GetEntryUri(), d.GetCompanionUris())
	}
	entries, err := os.ReadDir(filepath.Join(root, "mul"))
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
	if top, _ := os.ReadDir(root); len(top) != 1 {
		t.Errorf("store root holds %d entries after a replace, want only mul (no temp or trash left)", len(top))
	}
}

func TestDeletingADesignRemovesItsFolder(t *testing.T) {
	ctx := context.Background()
	ds, root := dirStore(t)
	mustDesign(t, ds, "mul", map[string]string{"design.yaml": "entry: a.edn\n", "a.edn": "(edif a)"})
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

func artifactURI(t *testing.T, s string) artifact.URI {
	t.Helper()
	u, err := artifact.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
