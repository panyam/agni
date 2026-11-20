package service_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/service"
)

// fakeMount is a service.MountReader over versions of a folder held in memory. refs names each ref's
// version, and branches the refs a sync follows. An empty version is a mount with no history, read
// from files[""]. It counts what it resolves and fetches.
type fakeMount struct {
	b        blobs
	refs     map[string]string
	branches map[string]bool
	files    map[string]map[string]string // version -> path -> contents
	resolves int
	fetches  []string
	// tags are the refs ListRefs lists, newest first, and branchAsked the branch it last listed
	// commits on.
	tags        []service.SourceRef
	branchAsked string
}

func (m *fakeMount) Resolve(_ context.Context, ref string) (string, bool, error) {
	m.resolves++
	if m.refs == nil {
		return "", true, nil
	}
	v, ok := m.refs[ref]
	if !ok {
		return "", false, service.ErrNotFound
	}
	return v, ref == "" || m.branches[ref], nil
}

func (m *fakeMount) List(_ context.Context, version, under string) ([]service.MountFile, error) {
	var out []service.MountFile
	for p, c := range m.files[version] {
		if under == "" || p == under || strings.HasPrefix(p, under+"/") {
			out = append(out, service.MountFile{Path: p, Size: int64(len(c))})
		}
	}
	slices.SortFunc(out, func(a, b service.MountFile) int { return strings.Compare(a.Path, b.Path) })
	return out, nil
}

func (m *fakeMount) Fetch(_ context.Context, version string, f service.MountFile) (string, bool, error) {
	c := m.files[version][f.Path]
	sum := sha256.Sum256([]byte(c))
	h := "sha256:" + hex.EncodeToString(sum[:])
	if _, ok := m.b[h]; ok {
		return h, false, nil
	}
	m.b[h] = []byte(c)
	m.fetches = append(m.fetches, f.Path)
	return h, true, nil
}

// kicadFolder is a KiCad design in hw/: a root sheet placing one child sheet, its project and its
// board, beside a README no reader opens.
func kicadFolder(power string) map[string]string {
	return map[string]string{
		"hw/bb.kicad_pro":           "{}",
		"hw/bb.kicad_sch":           `(kicad_sch (sheet (property "Sheetfile" "power.kicad_sch")))`,
		"hw/power.kicad_sch":        power,
		"hw/bb.kicad_pcb":           "(kicad_pcb)",
		"README.md":                 "notes",
		"other/unrelated.kicad_sch": "(kicad_sch)",
	}
}

// linkedService is a DesignService over a fresh folder store, linking to the mounts in readers, with
// the design bb created. It returns the folders the store keeps, so a test can open them again.
func linkedService(t *testing.T, b blobs, readers map[string]*fakeMount) (*service.DesignService, string, string) {
	t.Helper()
	root := t.TempDir()
	records, uploads := filepath.Join(root, "records"), filepath.Join(root, "uploads")
	svc := linkedServiceOver(t, b, readers, records, uploads)
	if _, err := svc.CreateDesign(context.Background(), &webapi.CreateDesignRequest{DesignId: "bb"}); err != nil {
		t.Fatal(err)
	}
	return svc, records, uploads
}

func linkedServiceOver(t *testing.T, b blobs, readers map[string]*fakeMount, records, uploads string) *service.DesignService {
	t.Helper()
	store, err := projects.NewDirStore(records, uploads, "uploads")
	if err != nil {
		t.Fatal(err)
	}
	return service.NewDesignService(service.NewDesignCatalog(store), b.source).WithLinks(func(_ context.Context, name string) (service.MountReader, error) {
		if r, ok := readers[name]; ok {
			return r, nil
		}
		if name == "uploads" {
			return nil, service.ErrInvalidArgument
		}
		return nil, service.ErrNotFound
	}, "uploads")
}

func link(t *testing.T, svc *service.DesignService, artifact, mount, path, ref string) *webapi.Artifact {
	t.Helper()
	a, err := svc.LinkArtifact(context.Background(), &webapi.LinkArtifactRequest{
		Name:         "designs/bb/artifacts/" + artifact,
		Source:       &webapi.ArtifactSource{Mount: mount, Path: path, Ref: ref},
		AllowMissing: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func paths(a *webapi.Artifact) []string {
	var out []string
	for _, f := range a.GetFiles() {
		out = append(out, f.GetPath())
	}
	slices.Sort(out)
	return out
}

func TestALinkedArtifactTakesItsDesignsFilesAndRefusesAPut(t *testing.T) {
	ctx := context.Background()
	b := blobs{}
	repo := &fakeMount{b: b, refs: map[string]string{"v1": "c1"}, files: map[string]map[string]string{"c1": kicadFolder("(kicad_sch)")}}
	svc, records, uploads := linkedService(t, b, map[string]*fakeMount{"repo": repo})

	sch := link(t, svc, "schematic", "repo", "hw/bb.kicad_sch", "v1")
	if got, want := paths(sch), []string{"bb.kicad_pro", "bb.kicad_sch", "power.kicad_sch"}; !slices.Equal(got, want) {
		t.Errorf("the schematic holds %v, want %v: the root sheet takes its child sheet and project, and leaves the board and other folders", got, want)
	}
	src := sch.GetSource()
	if sch.GetType() != webapi.ArtifactType_ARTIFACT_TYPE_SCHEMATIC || src.GetMount() != "repo" || src.GetVersion() != "c1" || src.GetFollow() || src.GetReadHash() == "" || src.GetReadTime() == nil {
		t.Errorf("the schematic = %v; want a schematic read from repo at c1, pinned, with its read time and hash", sch)
	}
	if got := paths(link(t, svc, "board", "repo", "hw/bb.kicad_pcb", "v1")); !slices.Equal(got, []string{"bb.kicad_pcb"}) {
		t.Errorf("the board holds %v, want the board file alone", got)
	}
	if got := paths(link(t, svc, "notes", "repo", "README.md", "v1")); !slices.Equal(got, []string{"README.md"}) {
		t.Errorf("a file no reader opens takes %v, want itself", got)
	}

	put := &webapi.UpdateArtifactRequest{Artifact: &webapi.Artifact{Name: "designs/bb/artifacts/schematic", Files: b.files(map[string]string{"x.kicad_sch": "(kicad_sch)"})}}
	if _, err := svc.UpdateArtifact(ctx, put); !errors.Is(err, service.ErrFailedPrecondition) || !strings.Contains(err.Error(), "repo/hw/bb.kicad_sch@v1") {
		t.Errorf("a put over a linked artifact = %v, want ErrFailedPrecondition naming its source", err)
	}
	batch := &webapi.BatchUpdateArtifactsRequest{Parent: "designs/bb", Requests: []*webapi.UpdateArtifactRequest{put}}
	if _, err := svc.BatchUpdateArtifacts(ctx, batch); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("a batch over a linked artifact = %v, want ErrFailedPrecondition", err)
	}
	folder := b.files(map[string]string{"bb.kicad_sch": "(kicad_sch)"})
	if _, err := svc.ImportDesignFiles(ctx, &webapi.ImportDesignFilesRequest{Parent: "designs/bb", Files: folder}); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("an import over a linked artifact = %v, want ErrFailedPrecondition", err)
	}

	// The record keeps the source, so a server reopened over the same folders reads it back.
	again := linkedServiceOver(t, b, map[string]*fakeMount{"repo": repo}, records, uploads)
	kept, err := again.GetArtifact(ctx, &webapi.GetArtifactRequest{Name: "designs/bb/artifacts/schematic"})
	if err != nil || kept.GetSource().GetVersion() != "c1" || kept.GetSource().GetPath() != "hw/bb.kicad_sch" || kept.GetSource().GetReadHash() != src.GetReadHash() {
		t.Errorf("after reopening, the schematic = %v, %v; want its source as linked", kept, err)
	}
	if names, err := again.LinkedTo(ctx, "repo"); err != nil || len(names) != 3 {
		t.Errorf("LinkedTo(repo) = %v, %v; want the three linked artifacts", names, err)
	}

	unlinked, err := svc.UnlinkArtifact(ctx, &webapi.UnlinkArtifactRequest{Name: "designs/bb/artifacts/schematic"})
	if err != nil || unlinked.GetSource().GetMount() != "uploads" || unlinked.GetSource().GetVersion() != "" || !slices.Equal(paths(unlinked), paths(sch)) {
		t.Fatalf("UnlinkArtifact = %v, %v; want the same files from the uploads mount", unlinked, err)
	}
	if _, err := svc.UpdateArtifact(ctx, put); err != nil {
		t.Errorf("a put after unlinking = %v, want it accepted", err)
	}
	if names, _ := svc.LinkedTo(ctx, "repo"); len(names) != 2 {
		t.Errorf("after unlinking the schematic, LinkedTo(repo) = %v; want the board and notes", names)
	}
}

func TestASyncMovesAFollowedLinkAndFetchesOnlyWhatChanged(t *testing.T) {
	ctx := context.Background()
	b := blobs{}
	repo := &fakeMount{
		b:        b,
		refs:     map[string]string{"main": "c1", "v1": "c1"},
		branches: map[string]bool{"main": true},
		files:    map[string]map[string]string{"c1": kicadFolder("(kicad_sch)"), "c2": kicadFolder("(kicad_sch (power))")},
	}
	svc, _, _ := linkedService(t, b, map[string]*fakeMount{"repo": repo})
	link(t, svc, "schematic", "repo", "hw/bb.kicad_sch", "main")
	link(t, svc, "board", "repo", "hw/bb.kicad_pcb", "main")
	link(t, svc, "pinned", "repo", "README.md", "v1")

	repo.refs["main"] = "c2"
	repo.fetches, repo.resolves = nil, 0
	dry, err := svc.SyncDesign(ctx, &webapi.SyncDesignRequest{Name: "designs/bb", ValidateOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := syncRows(dry); got["schematic"] != "c1->c2 changed" || got["pinned"] != "c1->c1" {
		t.Errorf("a dry run = %v; want the schematic to move to c2 and the pinned board to stay", got)
	}
	if len(repo.fetches) != 0 {
		t.Errorf("a dry run fetched %v, want nothing", repo.fetches)
	}
	if a, _ := svc.GetArtifact(ctx, &webapi.GetArtifactRequest{Name: "designs/bb/artifacts/schematic"}); a.GetSource().GetVersion() != "c1" {
		t.Errorf("after a dry run the schematic reads %q, want c1 still", a.GetSource().GetVersion())
	}

	repo.resolves = 0
	resp, err := svc.SyncDesign(ctx, &webapi.SyncDesignRequest{Name: "designs/bb"})
	if err != nil {
		t.Fatal(err)
	}
	if repo.resolves != 1 {
		t.Errorf("the sync resolved main %d times, want once for both artifacts on it", repo.resolves)
	}
	if !slices.Equal(repo.fetches, []string{"hw/power.kicad_sch"}) {
		t.Errorf("the sync fetched %v, want only the sheet that changed", repo.fetches)
	}
	rows := syncRows(resp)
	if rows["schematic"] != "c1->c2 changed fetched 1" || rows["board"] != "c1->c2" || rows["pinned"] != "c1->c1" {
		t.Errorf("the sync = %v; want the schematic changed with one file fetched, the board moved with none, and the pinned board left", rows)
	}
	for _, id := range []string{"schematic", "board"} {
		if a, _ := svc.GetArtifact(ctx, &webapi.GetArtifactRequest{Name: "designs/bb/artifacts/" + id}); a.GetSource().GetVersion() != "c2" {
			t.Errorf("after the sync %s reads %q, want c2", id, a.GetSource().GetVersion())
		}
	}

	repo.fetches = nil
	again, err := svc.SyncDesign(ctx, &webapi.SyncDesignRequest{Name: "designs/bb"})
	if err != nil {
		t.Fatal(err)
	}
	if got := syncRows(again); got["schematic"] != "c2->c2" || len(repo.fetches) != 0 {
		t.Errorf("a second sync = %v and fetched %v; want nothing changed or fetched", got, repo.fetches)
	}
}

func TestASyncOfAMountWithNoHistoryComparesItsFiles(t *testing.T) {
	ctx := context.Background()
	b := blobs{}
	disk := &fakeMount{b: b, files: map[string]map[string]string{"": {"job/a.txt": "a", "job/b.txt": "b"}}}
	svc, _, _ := linkedService(t, b, map[string]*fakeMount{"disk": disk})
	a := link(t, svc, "files", "disk", "job", "")
	if got := paths(a); !slices.Equal(got, []string{"a.txt", "b.txt"}) || !a.GetSource().GetFollow() || a.GetSource().GetVersion() != "" {
		t.Fatalf("a folder link = %v; want its files relative to it, followed, with no version", a)
	}
	resp, err := svc.SyncDesign(ctx, &webapi.SyncDesignRequest{Name: "designs/bb"})
	if err != nil || syncRows(resp)["files"] != "->" {
		t.Errorf("a sync of unchanged files = %v, %v; want unchanged", syncRows(resp), err)
	}
	disk.files[""]["job/b.txt"] = "b2"
	dry, err := svc.SyncDesign(ctx, &webapi.SyncDesignRequest{Name: "designs/bb", ValidateOnly: true})
	// A mount with no history names no version, so even a dry run reads the files, and keeps the
	// changed one.
	if err != nil || syncRows(dry)["files"] != "-> changed fetched 1" {
		t.Errorf("a dry run after a file changed = %v, %v; want changed with one file fetched", syncRows(dry), err)
	}
	if kept, _ := svc.GetArtifact(ctx, &webapi.GetArtifactRequest{Name: "designs/bb/artifacts/files"}); kept.GetSource().GetReadHash() != a.GetSource().GetReadHash() {
		t.Errorf("a dry run moved the read hash to %s; want it to write nothing", kept.GetSource().GetReadHash())
	}
	resp, err = svc.SyncDesign(ctx, &webapi.SyncDesignRequest{Name: "designs/bb", Artifacts: []string{"files"}})
	if err != nil || syncRows(resp)["files"] != "-> changed" {
		t.Errorf("a sync after the dry run = %v, %v; want changed, with nothing left to fetch", syncRows(resp), err)
	}
	after, _ := svc.GetArtifact(ctx, &webapi.GetArtifactRequest{Name: "designs/bb/artifacts/files"})
	if after.GetSource().GetReadHash() == a.GetSource().GetReadHash() {
		t.Errorf("the read hash stayed %s after the files changed", after.GetSource().GetReadHash())
	}
}

func TestLinkAndSyncRefuseWhatTheyCannotRead(t *testing.T) {
	ctx := context.Background()
	b := blobs{}
	repo := &fakeMount{b: b, refs: map[string]string{"v1": "c1"}, files: map[string]map[string]string{"c1": kicadFolder("(kicad_sch)")}}
	svc, _, _ := linkedService(t, b, map[string]*fakeMount{"repo": repo})
	putNetlist(t, svc, b, "bb", map[string]string{"a.edn": "(edif a)"})
	for _, c := range []struct {
		why  string
		src  *webapi.ArtifactSource
		want error
	}{
		{"an unknown mount", &webapi.ArtifactSource{Mount: "nope", Path: "x"}, service.ErrNotFound},
		{"the uploads mount", &webapi.ArtifactSource{Mount: "uploads", Path: "x"}, service.ErrInvalidArgument},
		{"no mount", &webapi.ArtifactSource{Path: "x"}, service.ErrInvalidArgument},
		{"a path climbing out", &webapi.ArtifactSource{Mount: "repo", Path: "../x", Ref: "v1"}, service.ErrInvalidArgument},
		{"a path the mount lacks", &webapi.ArtifactSource{Mount: "repo", Path: "hw/none.kicad_sch", Ref: "v1"}, service.ErrNotFound},
		{"a ref the mount lacks", &webapi.ArtifactSource{Mount: "repo", Path: "hw", Ref: "v9"}, service.ErrNotFound},
	} {
		_, err := svc.LinkArtifact(ctx, &webapi.LinkArtifactRequest{Name: "designs/bb/artifacts/x", Source: c.src, AllowMissing: true})
		if !errors.Is(err, c.want) {
			t.Errorf("a link to %s = %v, want %v", c.why, err, c.want)
		}
	}
	if _, err := svc.LinkArtifact(ctx, &webapi.LinkArtifactRequest{Name: "designs/bb/artifacts/x", Source: &webapi.ArtifactSource{Mount: "repo", Path: "hw", Ref: "v1"}}); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("a link to a missing artifact without allow_missing = %v, want ErrNotFound", err)
	}
	if _, err := svc.SyncDesign(ctx, &webapi.SyncDesignRequest{Name: "designs/bb", Artifacts: []string{"netlist"}}); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("a sync naming an uploaded artifact = %v, want ErrInvalidArgument", err)
	}
	plain := service.NewDesignService(nil, nil)
	if _, err := plain.LinkArtifact(ctx, &webapi.LinkArtifactRequest{Name: "designs/bb/artifacts/x", Source: &webapi.ArtifactSource{Mount: "repo"}}); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("a link on a server with no readers = %v, want ErrFailedPrecondition", err)
	}
}

// syncRows writes each artifact's sync as "from->to", then " changed" and " fetched N" when they
// apply, by artifact id.
func syncRows(resp *webapi.SyncDesignResponse) map[string]string {
	out := map[string]string{}
	for _, a := range resp.GetArtifacts() {
		row := a.GetFromVersion() + "->" + a.GetToVersion()
		if a.GetChanged() {
			row += " changed"
		}
		if a.GetFilesFetched() > 0 {
			row += " fetched " + string(rune('0'+a.GetFilesFetched()))
		}
		out[a.GetArtifact()[strings.LastIndex(a.GetArtifact(), "/")+1:]] = row
	}
	return out
}
