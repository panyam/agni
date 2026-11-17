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

func board(series string) map[string]string {
	desc := "entry: a.edn\n"
	if series != "" {
		desc += "series: " + series + "\n"
	}
	return map[string]string{"design.yaml": desc, "a.edn": "(edif a)"}
}

// setTime stamps a stored design's descriptor, so which design a series updated last is fixed.
func setTime(t *testing.T, root, id string, at time.Time) {
	t.Helper()
	if err := os.Chtimes(filepath.Join(root, id, "design.yaml"), at, at); err != nil {
		t.Fatal(err)
	}
}

func TestADesignIsCreatedReadListedReplacedAndDeleted(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := designService(t)
	d, err := svc.CreateDesign(ctx, &webapi.CreateDesignRequest{DesignId: "memory_unit", Files: b.files(board(""))})
	if err != nil {
		t.Fatal(err)
	}
	if d.GetName() != "designs/memory_unit" || d.GetEntryUri() != "mount://store/memory_unit/a.edn" || d.GetUpdateTime() == nil {
		t.Errorf("created = %s at %s, updated %v; want designs/memory_unit at mount://store/memory_unit/a.edn with an update time", d.GetName(), d.GetEntryUri(), d.GetUpdateTime())
	}
	got, err := svc.GetDesign(ctx, &webapi.GetDesignRequest{Name: "designs/memory_unit"})
	if err != nil || got.GetEntryUri() != d.GetEntryUri() {
		t.Errorf("GetDesign = %v, %v; want the created design", got, err)
	}
	list, err := svc.ListDesigns(ctx, &webapi.ListDesignsRequest{})
	if err != nil || len(list.GetDesigns()) != 2 || list.GetDesigns()[0].GetName() != "designs/g" || list.GetDesigns()[1].GetName() != "designs/memory_unit" {
		t.Fatalf("ListDesigns = %v, %v; want the mounted g, then memory_unit", list.GetDesigns(), err)
	}
	next := map[string]string{"design.yaml": "entry: b.edn\n", "b.edn": "(edif b)"}
	up, err := svc.UpdateDesign(ctx, &webapi.UpdateDesignRequest{Name: "designs/memory_unit", Files: b.files(next)})
	if err != nil || up.GetEntryUri() != "mount://store/memory_unit/b.edn" {
		t.Errorf("UpdateDesign = %v, %v; want the entry b.edn", up, err)
	}
	if err := svc.DeleteDesign(ctx, &webapi.DeleteDesignRequest{Name: "designs/memory_unit"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetDesign(ctx, &webapi.GetDesignRequest{Name: "designs/memory_unit"}); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("GetDesign after delete = %v, want ErrNotFound", err)
	}
}

func TestADesignServiceRefusesWhatItCannotWrite(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := designService(t)
	if _, err := svc.CreateDesign(ctx, &webapi.CreateDesignRequest{DesignId: "Bad/Id", Files: b.files(board(""))}); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("CreateDesign with a bad id = %v, want ErrInvalidArgument", err)
	}
	if _, err := svc.CreateDesign(ctx, &webapi.CreateDesignRequest{DesignId: "g", Files: b.files(board(""))}); !errors.Is(err, service.ErrAlreadyExists) {
		t.Errorf("CreateDesign over a mounted id = %v, want ErrAlreadyExists", err)
	}
	if _, err := svc.UpdateDesign(ctx, &webapi.UpdateDesignRequest{Name: "designs/g", Files: b.files(board(""))}); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("UpdateDesign of a mounted design = %v, want ErrFailedPrecondition", err)
	}
	if err := svc.DeleteDesign(ctx, &webapi.DeleteDesignRequest{Name: "designs/g"}); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("DeleteDesign of a mounted design = %v, want ErrFailedPrecondition", err)
	}
	if _, err := svc.GetDesign(ctx, &webapi.GetDesignRequest{Name: "projects/p/designs/g"}); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("GetDesign of a nested name = %v, want ErrInvalidArgument", err)
	}
	unsent := blobs{}.files(map[string]string{"design.yaml": "entry: u.edn\n", "u.edn": "(edif unsent)"})
	_, err := svc.CreateDesign(ctx, &webapi.CreateDesignRequest{DesignId: "mul", Files: unsent})
	var bm *service.BlobsMissingError
	if !errors.As(err, &bm) || len(bm.Hashes) != 2 {
		t.Errorf("CreateDesign with no contents sent = %v, want a BlobsMissingError naming both files", err)
	}
	none := service.NewDesignService(service.NewDesignCatalog(nil, projects.NewMountedDesigns(tutorial())), b.source)
	if _, err := none.CreateDesign(ctx, &webapi.CreateDesignRequest{DesignId: "mul", Files: b.files(board(""))}); !errors.Is(err, service.ErrFailedPrecondition) {
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
		if _, err := svc.CreateDesign(ctx, &webapi.CreateDesignRequest{DesignId: id, Files: b.files(board(series))}); err != nil {
			t.Fatal(err)
		}
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
	if _, err := svc.UpdateDesign(ctx, &webapi.UpdateDesignRequest{Name: "designs/mem-1.1.2", Files: b.files(board("memory_unit"))}); err != nil {
		t.Fatal(err)
	}
	if got := latest(); !got["designs/mem-1.1.2"] || got["designs/mem-1.1.3"] {
		t.Errorf("series after replacing 1.1.2 = %v, want 1.1.2 latest", got)
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
		if _, err := svc.CreateDesign(ctx, &webapi.CreateDesignRequest{DesignId: id, Files: b.files(board(""))}); err != nil {
			t.Fatal(err)
		}
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
