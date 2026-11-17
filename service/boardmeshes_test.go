package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/mesh"
	"github.com/panyam/agni/core/render"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// meshLoader serves a board and a sphere mesh for each key it knows, counting its reads.
type meshLoader struct {
	boardLoader
	meshes map[string]*mesh.Mesh
	reads  *int
}

func (l meshLoader) BoardMeshes(_ context.Context, _ artifact.URI, keys []string) (map[string]*mesh.Mesh, map[string]string, error) {
	*l.reads++
	out, unread := map[string]*mesh.Mesh{}, map[string]string{}
	for _, k := range keys {
		if m := l.meshes[k]; m != nil {
			out[k] = m
		} else {
			unread[k] = "not found"
		}
	}
	return out, unread, nil
}

// ball is a UV sphere with about 2n² triangles.
func ball(n int) *mesh.Mesh {
	var p mesh.Part
	for i := 0; i <= n; i++ {
		th := math.Pi * float64(i) / float64(n)
		for j := 0; j < n; j++ {
			ph := 2 * math.Pi * float64(j) / float64(n)
			p.Positions = append(p.Positions, [3]float64{math.Sin(th) * math.Cos(ph), math.Sin(th) * math.Sin(ph), math.Cos(th)})
		}
	}
	at := func(i, j int) uint32 { return uint32(i*n + j%n) }
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			p.Indices = append(p.Indices, at(i, j), at(i+1, j), at(i+1, j+1), at(i, j), at(i+1, j+1), at(i, j+1))
		}
	}
	p.Color = [4]float64{1, 0.5, 0, 1}
	return &mesh.Mesh{Parts: []mesh.Part{p}}
}

// meshBoard is a board of three parts. U1 is large and names a 5,000-triangle mesh, R1 and R2 are
// small and share a 1,250-triangle one, and C1 names a mesh no file holds.
func meshBoard(t *testing.T) (*DesignViewService, string, *int) {
	t.Helper()
	b := testBoard()
	b.UnitNm = 1
	pad := func(mm float64) []*geom.Pad {
		s := int64(mm * 1e6)
		return []*geom.Pad{{Number: "1", At: &geom.Point{}, Size: &geom.Point{X: s, Y: s}}}
	}
	inst := func(k string) []*geom.ModelInstance {
		return []*geom.ModelInstance{{Mesh: k, Transform: []float64{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0}}}
	}
	b.Placements = []*geom.ComponentPlacement{
		{RefDes: "U1", At: &geom.Point{}, Pads: pad(10), ModelInstances: inst("big.wrl")},
		{RefDes: "R1", At: &geom.Point{}, Pads: pad(1), ModelInstances: inst("small.wrl")},
		{RefDes: "R2", At: &geom.Point{}, Pads: pad(1), ModelInstances: inst("small.wrl")},
		{RefDes: "C1", At: &geom.Point{}, Pads: pad(1), ModelInstances: inst("gone.wrl")},
	}
	reads := 0
	ld := meshLoader{
		boardLoader: boardLoader{fakeLoader: fakeLoader{geom: &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{Id: "graph"}}}}, board: b},
		meshes:      map[string]*mesh.Mesh{"big.wrl": ball(50), "small.wrl": ball(25)},
		reads:       &reads,
	}
	svc := NewDesignViewService(ld, noNative{}, render.DefaultStyle, nil)
	return svc, schematicWithBoard(t, svc), &reads
}

func meshPages(t *testing.T, svc *DesignViewService, snap string, tris, bytes int64) []*webapi.GetBoardMeshesResponse {
	t.Helper()
	var pages []*webapi.GetBoardMeshesResponse
	token := ""
	for {
		resp, err := svc.GetBoardMeshes(context.Background(), &webapi.GetBoardMeshesRequest{Snapshot: snap, MaxTriangles: tris, MaxBytes: bytes, PageToken: token})
		if err != nil {
			t.Fatalf("page %d: %v", len(pages), err)
		}
		pages = append(pages, resp)
		if token = resp.GetNextPageToken(); token == "" {
			return pages
		}
		if len(pages) > 100 {
			t.Fatal("paging did not end")
		}
	}
}

func meshTriangles(m *geom.ModelMesh) int {
	n := 0
	for _, p := range m.GetParts() {
		n += len(p.GetIndices()) / 3
	}
	return n
}

func TestGetBoardMeshesReturnsEachMeshOnceWithinTheDefaultBudget(t *testing.T) {
	svc, snap, _ := meshBoard(t)
	pages := meshPages(t, svc, snap, 0, 0)
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(pages))
	}
	p := pages[0]
	got := map[string]*geom.ModelMesh{}
	for _, m := range p.GetMeshes() {
		got[m.GetKey()] = m
	}
	if len(got) != 2 || got["big.wrl"] == nil || got["small.wrl"] == nil {
		t.Fatalf("meshes %v, want big.wrl and small.wrl once each", got)
	}
	if n := meshTriangles(got["big.wrl"]); n != 5000 || got["big.wrl"].GetSourceTriangles() != 5000 {
		t.Errorf("big.wrl has %d of %d triangles, want all 5000", n, got["big.wrl"].GetSourceTriangles())
	}
	if p.GetTriangles() != 6250 || len(p.GetSimplified()) != 0 || len(p.GetBlocks()) != 0 {
		t.Errorf("triangles %d, simplified %v, blocks %v, want 6250 and none", p.GetTriangles(), p.GetSimplified(), p.GetBlocks())
	}
	if u := p.GetUnread(); len(u) != 1 || u[0].GetPath() != "gone.wrl" || u[0].GetParts() != 1 {
		t.Errorf("unread %v, want gone.wrl for its one part", u)
	}
	if rgba := got["small.wrl"].GetParts()[0].GetRgba(); rgba != 0xff8000ff {
		t.Errorf("rgba %08x, want ff8000ff", rgba)
	}
}

func TestGetBoardMeshesSimplifiesTheLargestThenBlocksTheSmallest(t *testing.T) {
	svc, snap, _ := meshBoard(t)
	p := meshPages(t, svc, snap, 3000, 0)[0]
	if fmt.Sprint(p.GetSimplified()) != "[big.wrl]" || p.GetTriangles() > 3000 || len(p.GetBlocks()) != 0 {
		t.Errorf("at 3000: simplified %v, %d triangles, blocks %v, want big.wrl cut and nothing blocked", p.GetSimplified(), p.GetTriangles(), p.GetBlocks())
	}
	p = meshPages(t, svc, snap, int64(mesh.MinTriangles), 0)[0]
	if fmt.Sprint(p.GetBlocks()) != "[small.wrl]" || len(p.GetMeshes()) != 1 || p.GetMeshes()[0].GetKey() != "big.wrl" {
		t.Errorf("at %d: blocks %v and %d meshes, want the small parts' mesh blocked and big.wrl kept", mesh.MinTriangles, p.GetBlocks(), len(p.GetMeshes()))
	}
}

func TestGetBoardMeshesPagesReadTheModelsOnce(t *testing.T) {
	svc, snap, reads := meshBoard(t)
	pages := meshPages(t, svc, snap, 0, 1)
	if len(pages) != 2 {
		t.Fatalf("at 1 byte a page got %d pages, want one per mesh", len(pages))
	}
	if pages[1].GetTriangles() != 0 || len(pages[1].GetUnread()) != 0 {
		t.Errorf("the second page repeats the summary: %d triangles, unread %v", pages[1].GetTriangles(), pages[1].GetUnread())
	}
	if *reads != 1 {
		t.Errorf("two pages read the models %d times, want once", *reads)
	}
	_, err := svc.GetBoardMeshes(context.Background(), &webapi.GetBoardMeshesRequest{Snapshot: snap, MaxTriangles: 1000, MaxBytes: 1, PageToken: pages[0].GetNextPageToken()})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("a token from another budget gave %v, want invalid_argument", err)
	}
}

func TestGetBoardMeshesWithoutAMeshLoaderListsEveryMeshUnread(t *testing.T) {
	b := testBoard()
	b.Placements = []*geom.ComponentPlacement{{RefDes: "U1", At: &geom.Point{}, ModelInstances: []*geom.ModelInstance{{Mesh: "a.wrl"}}}}
	svc := NewDesignViewService(boardLoader{fakeLoader: fakeLoader{geom: &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{Id: "graph"}}}}, board: b}, noNative{}, render.DefaultStyle, nil)
	p := meshPages(t, svc, schematicWithBoard(t, svc), 0, 0)[0]
	if len(p.GetMeshes()) != 0 || len(p.GetUnread()) != 1 || p.GetUnread()[0].GetReason() != "this server reads no model meshes" {
		t.Errorf("meshes %v, unread %v, want a.wrl unread", p.GetMeshes(), p.GetUnread())
	}
}
