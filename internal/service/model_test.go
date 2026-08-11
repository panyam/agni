package service

import (
	"context"
	"errors"
	"github.com/panyam/agni/internal/artifact"
	"testing"

	"github.com/panyam/agni/core/check"
	param "github.com/panyam/agni/datasheet/param"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// thinBoard is a one-net board whose only track is 0.05mm wide — below the 0.127mm fab floor, so
// track-width fires on it. Enough to prove a board tier reached the model.
func thinBoard() *geom.BoardGeometry {
	pt := func(x, y int64) *geom.Point { return &geom.Point{X: x, Y: y} }
	return &geom.BoardGeometry{UnitNm: 1, Nets: []*geom.NetCopper{{
		Net:      "THIN",
		Segments: []*geom.TrackSegment{{A: pt(0, 0), B: pt(4_000_000, 0), Width: 50_000, Layer: "F.Cu"}},
	}}}
}

// someSpecs is a non-nil param provider (contents irrelevant here — HasParams reports that a
// provider was attached, not that a part is seeded).
func someSpecs() param.ParamProvider {
	return param.ProviderFunc(func(string) *parampb.PartSpec { return nil })
}

// TestBuildModelAttachesTiers: BuildModel attaches BOTH the board and params tiers, the drift
// WS9-048 closes (services were building plain netlist models that dropped them).
func TestBuildModelAttachesTiers(t *testing.T) {
	m, err := BuildModel(context.Background(), fakeLoader{design: &ir.Design{}, board: thinBoard()}, testURI(t, "m", "d"), artifact.URI{}, someSpecs())
	if err != nil {
		t.Fatalf("BuildModel: %v", err)
	}
	if !m.HasBoard() {
		t.Error("HasBoard() = false; board tier not attached")
	}
	if !m.HasParams() {
		t.Error("HasParams() = false; params tier not attached")
	}
}

// TestBuildModelBoardPathNonBoard: a board_path override that reads no board is a loud error, never
// a silent nil (WS3-089), so an explicit board request can't report clean without checking.
func TestBuildModelBoardPathNonBoard(t *testing.T) {
	// The fake returns a nil board for the override path; BuildModel must reject it.
	_, err := BuildModel(context.Background(), fakeLoader{design: &ir.Design{}, board: nil}, testURI(t, "m", "d"), testURI(t, "m", "board.override"), someSpecs())
	if err == nil {
		t.Fatal("board_path with no board did not error")
	}
}

// TestBuildGeometryBestEffort: BuildGeometry passes a loaded geometry through, and returns nil (not an
// error) when the load fails — the best-effort contract that lets a caller degrade to "no sheet badges"
// rather than fail. This is why geometry is a separate helper, not a BuildModel tier.
func TestBuildGeometryBestEffort(t *testing.T) {
	g := &geom.SchematicGeometry{}
	if got := BuildGeometry(context.Background(), fakeLoader{geom: g}, testURI(t, "m", "d")); got != g {
		t.Errorf("loaded geometry = %v, want the passed value", got)
	}
	if got := BuildGeometry(context.Background(), fakeLoader{geomErr: errors.New("boom")}, testURI(t, "m", "d")); got != nil {
		t.Errorf("failed load = %v, want nil (best-effort)", got)
	}
}

// TestCheckDesignRunsBoardRules: served CheckDesign fires a board-DRC rule when the design carries
// board geometry — the observable bug WS9-048 fixes (CheckDesign built check.NewModel(d), so
// check.Available gated every board.* rule to not-applicable and the panel showed nothing).
func TestCheckDesignRunsBoardRules(t *testing.T) {
	svc := NewCheckService(fakeLoader{design: &ir.Design{}, board: thinBoard()}, check.DefaultCatalog(), nil, "", nil, nil)
	resp, err := svc.CheckDesign(context.Background(), &webapi.CheckDesignRequest{Uri: "mount://m/d", Rules: []string{"track-width"}})
	if err != nil {
		t.Fatalf("CheckDesign: %v", err)
	}
	found := false
	for _, f := range resp.GetFindings() {
		if f.GetRule() == "track-width" {
			found = true
		}
	}
	if !found {
		t.Errorf("track-width did not fire on a board-bearing design; findings = %+v", resp.GetFindings())
	}
}

// testURI builds an artifact URI for a test, failing rather than returning an error: a hard-coded
// fixture URI that will not parse is a broken test, not a condition under test.
func testURI(t *testing.T, mount, p string) artifact.URI {
	t.Helper()
	u, err := artifact.New(mount, p)
	if err != nil {
		t.Fatalf("artifact.New(%q, %q): %v", mount, p, err)
	}
	return u
}
