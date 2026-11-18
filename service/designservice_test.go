package service_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/service"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// blobs is an upload store in a test: contents by hash.
type blobs map[string][]byte

func (b blobs) source(_ context.Context, h string) ([]byte, bool) { v, ok := b[h]; return v, ok }

// files turns a design's files into the manifest a client sends, keeping their contents in b.
func (b blobs) files(contents map[string]string) []*webapi.SnapshotFile {
	var out []*webapi.SnapshotFile
	for p, c := range contents {
		sum := sha256.Sum256([]byte(c))
		h := "sha256:" + hex.EncodeToString(sum[:])
		b[h] = []byte(c)
		out = append(out, &webapi.SnapshotFile{Path: p, Sha256: h, Size: int64(len(c))})
	}
	return out
}

// tutorial is a mounted folder holding one design, g.
func tutorial() projects.Tree {
	return projects.Tree{Mount: "tut", FS: fstest.MapFS{
		"g/design.yaml": {Data: []byte("name: g\nentry: g.edn\n")},
		"g/g.edn":       {Data: []byte("(edif g)")},
	}}
}

// designService is a DesignService over a fresh folder store and the tutorial mount.
func designService(t *testing.T) (*service.DesignService, blobs, string) {
	t.Helper()
	root := t.TempDir()
	store, err := projects.NewDirStore(root, "store")
	if err != nil {
		t.Fatal(err)
	}
	b := blobs{}
	svc := service.NewDesignService(service.NewDesignCatalog(store, projects.NewMountedDesigns(tutorial())), b.source)
	return svc, b, root
}

// newDesign creates a design in series and puts one netlist artifact holding contents, so it reads.
func newDesign(t *testing.T, svc *service.DesignService, b blobs, id, series, contents string) {
	t.Helper()
	ctx := context.Background()
	if _, err := svc.CreateDesign(ctx, &webapi.CreateDesignRequest{DesignId: id, Design: &webapi.Design{Series: series}}); err != nil {
		t.Fatal(err)
	}
	putNetlist(t, svc, b, id, map[string]string{"a.edn": contents})
}

// putNetlist puts the netlist artifact of a design, creating it if missing.
func putNetlist(t *testing.T, svc *service.DesignService, b blobs, id string, files map[string]string) *webapi.Artifact {
	t.Helper()
	a, err := svc.UpdateArtifact(context.Background(), &webapi.UpdateArtifactRequest{
		Artifact:     &webapi.Artifact{Name: "designs/" + id + "/artifacts/netlist", Files: b.files(files)},
		AllowMissing: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// setTime stamps a stored design's descriptor, so which design a series updated last is fixed.
func setTime(t *testing.T, root, id string, at time.Time) {
	t.Helper()
	if err := os.Chtimes(filepath.Join(root, id, "design.yaml"), at, at); err != nil {
		t.Fatal(err)
	}
}

func TestADesignIsCreatedReadListedChangedAndDeleted(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := designService(t)
	d, err := svc.CreateDesign(ctx, &webapi.CreateDesignRequest{DesignId: "memory_unit", Design: &webapi.Design{Title: "Memory unit", Series: "mem"}})
	if err != nil {
		t.Fatal(err)
	}
	if d.GetName() != "designs/memory_unit" || d.GetTitle() != "Memory unit" || d.GetSeries() != "mem" || d.GetEntryUri() != "" || d.GetUpdateTime() == nil {
		t.Errorf("created = %v; want designs/memory_unit with its title and series, no entry yet, and an update time", d)
	}
	a := putNetlist(t, svc, b, "memory_unit", map[string]string{"a.edn": "(edif a)"})
	if a.GetName() != "designs/memory_unit/artifacts/netlist" || a.GetType() != webapi.ArtifactType_ARTIFACT_TYPE_NETLIST || a.GetEntry() != "a.edn" {
		t.Errorf("put artifact = %v; want designs/memory_unit/artifacts/netlist, a netlist entered at a.edn", a)
	}
	got, err := svc.GetDesign(ctx, &webapi.GetDesignRequest{Name: "designs/memory_unit"})
	if err != nil || got.GetEntryUri() != "mount://store/memory_unit/netlist/a.edn" || got.GetTitle() != "Memory unit" {
		t.Errorf("GetDesign = %v, %v; want the netlist as entry and the title kept", got, err)
	}
	list, err := svc.ListDesigns(ctx, &webapi.ListDesignsRequest{})
	if err != nil || len(list.GetDesigns()) != 2 || list.GetDesigns()[0].GetName() != "designs/g" || list.GetDesigns()[1].GetName() != "designs/memory_unit" {
		t.Fatalf("ListDesigns = %v, %v; want the mounted g, then memory_unit", list.GetDesigns(), err)
	}
	up, err := svc.UpdateDesign(ctx, &webapi.UpdateDesignRequest{
		Design:     &webapi.Design{Name: "designs/memory_unit", Title: "ignored", Series: "memory_unit"},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"series"}},
	})
	if err != nil || up.GetSeries() != "memory_unit" || up.GetTitle() != "Memory unit" || up.GetEntryUri() != got.GetEntryUri() {
		t.Errorf("UpdateDesign of the series = %v, %v; want the series changed, the title and entry kept", up, err)
	}
	if _, err := svc.UpdateDesign(ctx, &webapi.UpdateDesignRequest{Design: &webapi.Design{Name: "designs/memory_unit"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"entry_uri"}}}); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("UpdateDesign of entry_uri = %v, want ErrInvalidArgument", err)
	}
	if err := svc.DeleteDesign(ctx, &webapi.DeleteDesignRequest{Name: "designs/memory_unit"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetDesign(ctx, &webapi.GetDesignRequest{Name: "designs/memory_unit"}); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("GetDesign after delete = %v, want ErrNotFound", err)
	}
}

func TestArtifactsAreReadListedReplacedAndDeleted(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := designService(t)
	newDesign(t, svc, b, "mul", "", "(edif a)")
	if _, err := svc.UpdateArtifact(ctx, &webapi.UpdateArtifactRequest{Artifact: &webapi.Artifact{Name: "designs/mul/artifacts/board", Files: b.files(map[string]string{"a.kicad_pcb": "(kicad_pcb)"})}}); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("UpdateArtifact of a missing artifact without allow_missing = %v, want ErrNotFound", err)
	}
	resp, err := svc.BatchUpdateArtifacts(ctx, &webapi.BatchUpdateArtifactsRequest{Parent: "designs/mul", Requests: []*webapi.UpdateArtifactRequest{
		{Artifact: &webapi.Artifact{Name: "designs/mul/artifacts/netlist", Files: b.files(map[string]string{"b.edn": "(edif b)"})}},
		{Artifact: &webapi.Artifact{Name: "designs/mul/artifacts/board", Files: b.files(map[string]string{"b.kicad_pcb": "(kicad_pcb b)"})}, AllowMissing: true},
	}})
	if err != nil || len(resp.GetArtifacts()) != 2 || resp.GetArtifacts()[0].GetEntry() != "b.edn" || resp.GetArtifacts()[1].GetType() != webapi.ArtifactType_ARTIFACT_TYPE_BOARD {
		t.Fatalf("BatchUpdateArtifacts = %v, %v; want the netlist replaced and the board created, in request order", resp.GetArtifacts(), err)
	}
	list, err := svc.ListArtifacts(ctx, &webapi.ListArtifactsRequest{Parent: "designs/mul"})
	if err != nil || len(list.GetArtifacts()) != 2 || list.GetArtifacts()[0].GetName() != "designs/mul/artifacts/board" {
		t.Errorf("ListArtifacts = %v, %v; want board then netlist", list.GetArtifacts(), err)
	}
	if a, err := svc.GetArtifact(ctx, &webapi.GetArtifactRequest{Name: "designs/mul/artifacts/netlist"}); err != nil || len(a.GetFiles()) != 1 || a.GetFiles()[0].GetPath() != "b.edn" {
		t.Errorf("GetArtifact(netlist) = %v, %v; want only b.edn", a, err)
	}
	if err := svc.DeleteArtifact(ctx, &webapi.DeleteArtifactRequest{Name: "designs/mul/artifacts/board"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetArtifact(ctx, &webapi.GetArtifactRequest{Name: "designs/mul/artifacts/board"}); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("GetArtifact after delete = %v, want ErrNotFound", err)
	}
	if list, err := svc.ListArtifacts(ctx, &webapi.ListArtifactsRequest{Parent: "designs/g"}); err != nil || len(list.GetArtifacts()) != 0 {
		t.Errorf("ListArtifacts of the mounted g = %v, %v; want none", list.GetArtifacts(), err)
	}
}

func TestABatchIsRefusedWhole(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := designService(t)
	newDesign(t, svc, b, "mul", "", "(edif a)")
	newDesign(t, svc, b, "other", "", "(edif o)")
	board := &webapi.Artifact{Name: "designs/mul/artifacts/board", Files: b.files(map[string]string{"a.kicad_pcb": "(kicad_pcb)"})}
	for name, reqs := range map[string][]*webapi.UpdateArtifactRequest{
		"no requests":               nil,
		"one artifact twice":        {{Artifact: board, AllowMissing: true}, {Artifact: board, AllowMissing: true}},
		"another design's":          {{Artifact: &webapi.Artifact{Name: "designs/other/artifacts/netlist", Files: board.GetFiles()}}},
		"an artifact with no files": {{Artifact: &webapi.Artifact{Name: "designs/mul/artifacts/empty"}, AllowMissing: true}},
	} {
		if _, err := svc.BatchUpdateArtifacts(ctx, &webapi.BatchUpdateArtifactsRequest{Parent: "designs/mul", Requests: reqs}); !errors.Is(err, service.ErrInvalidArgument) {
			t.Errorf("a batch with %s = %v, want ErrInvalidArgument", name, err)
		}
	}
	// The second request names a missing artifact without allow_missing, so the first is not written.
	_, err := svc.BatchUpdateArtifacts(ctx, &webapi.BatchUpdateArtifactsRequest{Parent: "designs/mul", Requests: []*webapi.UpdateArtifactRequest{
		{Artifact: board, AllowMissing: true},
		{Artifact: &webapi.Artifact{Name: "designs/mul/artifacts/bom", Files: b.files(map[string]string{"bom.csv": "ref\n"})}},
	}})
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("a batch naming a missing artifact = %v, want ErrNotFound", err)
	}
	if list, err := svc.ListArtifacts(ctx, &webapi.ListArtifactsRequest{Parent: "designs/mul"}); err != nil || len(list.GetArtifacts()) != 1 {
		t.Errorf("artifacts after the refused batch = %v, %v; want only the netlist", list.GetArtifacts(), err)
	}
}

func TestADesignServiceRefusesWhatItCannotWrite(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := designService(t)
	if _, err := svc.CreateDesign(ctx, &webapi.CreateDesignRequest{DesignId: "Bad/Id"}); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("CreateDesign with a bad id = %v, want ErrInvalidArgument", err)
	}
	if _, err := svc.CreateDesign(ctx, &webapi.CreateDesignRequest{DesignId: "g"}); !errors.Is(err, service.ErrAlreadyExists) {
		t.Errorf("CreateDesign over a mounted id = %v, want ErrAlreadyExists", err)
	}
	if _, err := svc.UpdateDesign(ctx, &webapi.UpdateDesignRequest{Design: &webapi.Design{Name: "designs/g", Series: "s"}}); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("UpdateDesign of a mounted design = %v, want ErrFailedPrecondition", err)
	}
	if _, err := svc.UpdateArtifact(ctx, &webapi.UpdateArtifactRequest{Artifact: &webapi.Artifact{Name: "designs/g/artifacts/netlist", Files: b.files(map[string]string{"a.edn": "(edif a)"})}, AllowMissing: true}); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("UpdateArtifact of a mounted design = %v, want ErrFailedPrecondition", err)
	}
	if err := svc.DeleteDesign(ctx, &webapi.DeleteDesignRequest{Name: "designs/g"}); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("DeleteDesign of a mounted design = %v, want ErrFailedPrecondition", err)
	}
	if _, err := svc.GetDesign(ctx, &webapi.GetDesignRequest{Name: "projects/p/designs/g"}); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("GetDesign of a nested name = %v, want ErrInvalidArgument", err)
	}
	for _, name := range []string{"designs/mul/artifacts/Bad", "designs/mul/netlist", "designs/mul/artifacts/a/b"} {
		if _, err := svc.UpdateArtifact(ctx, &webapi.UpdateArtifactRequest{Artifact: &webapi.Artifact{Name: name, Files: b.files(map[string]string{"a.edn": "x"})}, AllowMissing: true}); !errors.Is(err, service.ErrInvalidArgument) {
			t.Errorf("UpdateArtifact(%s) = %v, want ErrInvalidArgument", name, err)
		}
	}
	if _, err := svc.CreateDesign(ctx, &webapi.CreateDesignRequest{DesignId: "mul"}); err != nil {
		t.Fatal(err)
	}
	unsent := blobs{}.files(map[string]string{"u.edn": "(edif unsent)", "u.kicad_pcb": "(kicad_pcb unsent)"})
	_, err := svc.UpdateArtifact(ctx, &webapi.UpdateArtifactRequest{Artifact: &webapi.Artifact{Name: "designs/mul/artifacts/netlist", Files: unsent}, AllowMissing: true})
	var bm *service.BlobsMissingError
	if !errors.As(err, &bm) || len(bm.Hashes) != 2 {
		t.Errorf("UpdateArtifact with no contents sent = %v, want a BlobsMissingError naming both files", err)
	}
	none := service.NewDesignService(service.NewDesignCatalog(nil, projects.NewMountedDesigns(tutorial())), b.source)
	if _, err := none.CreateDesign(ctx, &webapi.CreateDesignRequest{DesignId: "mul"}); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("CreateDesign with no store = %v, want ErrFailedPrecondition", err)
	}
}

func TestASeriesMarksTheDesignItUpdatedLast(t *testing.T) {
	ctx := context.Background()
	svc, b, root := designService(t)
	for _, id := range []string{"mem-1.1.2", "mem-1.1.3", "other"} {
		series := "memory_unit"
		if id == "other" {
			series = ""
		}
		newDesign(t, svc, b, id, series, "(edif "+id+")")
	}
	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	setTime(t, root, "mem-1.1.2", base)
	setTime(t, root, "mem-1.1.3", base.Add(time.Hour))
	latest := func() map[string]bool {
		t.Helper()
		list, err := svc.ListDesigns(ctx, &webapi.ListDesignsRequest{Filter: `series = "memory_unit"`})
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, d := range list.GetDesigns() {
			out[d.GetName()] = d.GetSeriesLatest()
		}
		return out
	}
	if got := latest(); len(got) != 2 || !got["designs/mem-1.1.3"] || got["designs/mem-1.1.2"] {
		t.Errorf("series = %v, want both members and 1.1.3 latest", got)
	}
	putNetlist(t, svc, b, "mem-1.1.2", map[string]string{"a.edn": "(edif 1.1.2 fixed)"})
	if got := latest(); !got["designs/mem-1.1.2"] || got["designs/mem-1.1.3"] {
		t.Errorf("series after replacing 1.1.2's netlist = %v, want 1.1.2 latest", got)
	}
	d, err := svc.GetDesign(ctx, &webapi.GetDesignRequest{Name: "designs/mem-1.1.2"})
	if err != nil || !d.GetSeriesLatest() {
		t.Errorf("GetDesign(mem-1.1.2) = latest %v, %v; want latest", d.GetSeriesLatest(), err)
	}
	if d, err := svc.GetDesign(ctx, &webapi.GetDesignRequest{Name: "designs/other"}); err != nil || d.GetSeriesLatest() {
		t.Errorf("a design with no series = latest %v, %v; want false", d.GetSeriesLatest(), err)
	}
}

func TestListDesignsFiltersByProjectAndRefusesOtherFilters(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := designService(t)
	for _, id := range []string{"a", "b"} {
		newDesign(t, svc, b, id, "", "(edif "+id+")")
	}
	ps := service.NewProjectService(nil).WithRecords(service.NewMemProjectRecords())
	if _, err := ps.CreateProject(ctx, &webapi.CreateProjectRequest{ProjectId: "b200", Project: &webapi.Project{Designs: []string{"designs/b", "designs/g", "designs/gone"}}}); err != nil {
		t.Fatal(err)
	}
	svc.WithProjects(ps)
	list, err := svc.ListDesigns(ctx, &webapi.ListDesignsRequest{Filter: `project = "projects/b200"`})
	if err != nil || len(list.GetDesigns()) != 2 || list.GetDesigns()[0].GetName() != "designs/b" || list.GetDesigns()[1].GetName() != "designs/g" {
		t.Errorf("project filter = %v, %v; want designs/b and designs/g", list.GetDesigns(), err)
	}
	if _, err := svc.ListDesigns(ctx, &webapi.ListDesignsRequest{Filter: `project = "projects/none"`}); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("filter on a missing project = %v, want ErrNotFound", err)
	}
	for _, f := range []string{`title = "x"`, `series`, `series = memory_unit`} {
		if _, err := svc.ListDesigns(ctx, &webapi.ListDesignsRequest{Filter: f}); !errors.Is(err, service.ErrInvalidArgument) {
			t.Errorf("filter %q = %v, want ErrInvalidArgument", f, err)
		}
	}
}

func TestListDesignsPagesAndReportsUnreachable(t *testing.T) {
	ctx := context.Background()
	copyOfG := projects.Tree{Mount: "copy", FS: fstest.MapFS{
		"g/design.yaml": {Data: []byte("name: g\nentry: g.edn\n")},
		"g/g.edn":       {Data: []byte("(edif g)")},
		"h/design.yaml": {Data: []byte("name: h\nentry: h.edn\n")},
		"h/h.edn":       {Data: []byte("(edif h)")},
		"k/design.yaml": {Data: []byte("name: k\nentry: k.edn\n")},
		"k/k.edn":       {Data: []byte("(edif k)")},
	}}
	svc := service.NewDesignService(service.NewDesignCatalog(nil, projects.NewMountedDesigns(tutorial(), copyOfG)), nil)
	var names []string
	for token, pages := "", 0; ; pages++ {
		if pages > 10 {
			t.Fatalf("paging has not ended after %d pages, at %v", pages, names)
		}
		page, err := svc.ListDesigns(ctx, &webapi.ListDesignsRequest{PageSize: 1, PageToken: token})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.GetUnreachable()) != 1 || page.GetUnreachable()[0] != "designs/g" {
			t.Errorf("unreachable = %v, want designs/g", page.GetUnreachable())
		}
		for _, d := range page.GetDesigns() {
			names = append(names, d.GetName())
		}
		if token = page.GetNextPageToken(); token == "" {
			break
		}
	}
	if len(names) != 2 || names[0] != "designs/h" || names[1] != "designs/k" {
		t.Errorf("paged designs = %v, want designs/h then designs/k", names)
	}
}
