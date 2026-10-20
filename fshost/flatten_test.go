package fshost

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/service"
)

const hierBlocks = "mount://m/hier_blocks.edn"

type flattenFixture struct {
	snaps *service.SnapshotService
	check *service.CheckService
	query *service.QueryService
	diff  *service.DiffService
}

// newFlattenFixture serves the EDIF reader's fixtures through ONE caching loader, so a flattened and
// an unflattened snapshot of one file share every cache tier and only the keys can keep them apart.
func newFlattenFixture(t *testing.T) *flattenFixture {
	t.Helper()
	ms := []Mount{{Name: "m", FS: os.DirFS(filepath.Join("..", "readers", "edif", "testdata"))}}
	res := &service.ProjectResolver{Store: projects.NewFSStore(projects.Tree{Mount: "m", FS: ms[0].FS}), Config: &ConfigResolver{Mounts: ms}}
	loader := service.NewCachingLoader(New(ms...), service.NewDesignCache(32))
	chk := service.NewCheckService(loader, check.DefaultCatalog(), nil, "", nil, res)
	return &flattenFixture{snaps: chk.Snapshots(), check: chk, query: service.NewQueryService(loader, nil, res), diff: service.NewDiffService(loader, res)}
}

func (f *flattenFixture) ingest(t *testing.T, flatten bool) string {
	t.Helper()
	s, err := f.snaps.Ingest(context.Background(), &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: hierBlocks, FlattenHierarchy: flatten}})
	if err != nil {
		t.Fatal(err)
	}
	return s.GetName()
}

func (f *flattenFixture) components(t *testing.T, snap string) int {
	t.Helper()
	r, err := f.query.RunQuery(context.Background(), &webapi.RunQueryRequest{Snapshot: snap, Query: "component.net_count(?r, ?n) => ?r"})
	if err != nil {
		t.Fatal(err)
	}
	return len(r.GetRows())
}

// TestFlattenedSnapshotIsOneModelAcrossServices holds a snapshot's flatten_hierarchy to every
// analysis rpc, checks, queries and diff, over one cache that also holds the unflattened read.
func TestFlattenedSnapshotIsOneModelAcrossServices(t *testing.T) {
	ctx := context.Background()
	f := newFlattenFixture(t)
	top := f.ingest(t, false)
	flat := f.ingest(t, true)

	// The top cell's blocks carry no designator, so unflattened only U5 and J1 have a ref-des; any
	// count but 8 tells the two reads apart.
	if n := f.components(t, top); n == 8 {
		t.Errorf("unflattened query sees %d components, want the top cell's alone", n)
	}
	if n := f.components(t, flat); n != 8 {
		t.Errorf("flattened query sees %d components, want 8", n)
	}
	// Asked again in the other order, so an answer kept under the wrong key shows either way.
	if n := f.components(t, top); n == 8 {
		t.Errorf("unflattened query after a flattened one sees the flattened %d components", n)
	}

	for snap, want := range map[string]int{top: 2, flat: 0} {
		run, err := f.check.CheckDesign(ctx, &webapi.CheckDesignRequest{Snapshot: snap})
		if err != nil {
			t.Fatal(err)
		}
		if got := len(run.GetUnexpandedHierarchy()); got != want {
			t.Errorf("CheckDesign on %s lists %d unexpanded cells, want %d", snap, got, want)
		}
	}

	d, err := f.diff.DiffDesigns(ctx, &webapi.DiffDesignsRequest{ASnapshot: top, BSnapshot: flat})
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"R1", "D1", "B1/R1", "B2/C1"} {
		if got := d.GetComponentStatus()[ref]; got == "" || got == "unchanged" {
			t.Errorf("diff top -> flattened gives %s status %q, want it added", ref, got)
		}
	}
	same, err := f.diff.DiffDesigns(ctx, &webapi.DiffDesignsRequest{ASnapshot: flat, BSnapshot: f.ingest(t, true)})
	if err != nil {
		t.Fatal(err)
	}
	for ref, st := range same.GetComponentStatus() {
		if st != "unchanged" && st != "" {
			t.Errorf("two flattened ingests differ on %s: %s", ref, st)
		}
	}
}
