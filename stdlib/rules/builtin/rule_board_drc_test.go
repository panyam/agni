package builtin

import (
	"fmt"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// mm builds a nanometer point from millimeter coordinates.
func mm(x, y float64) *geom.Point {
	return &geom.Point{X: int64(x * 1e6), Y: int64(y * 1e6)}
}

// drcBoard: every violation class once, on its own net, plus clean copper that must not
// fire: a sub-floor trace, a cross-net pair 0.04mm apart edge-to-edge, a pair the same
// distance apart on DIFFERENT layers, a same-net close pair, a sub-floor drill, and a
// sub-floor annular ring.
func drcBoard() *geom.BoardGeometry {
	seg := func(x1, y1, x2, y2, wMM float64, layer string) *geom.TrackSegment {
		return &geom.TrackSegment{A: mm(x1, y1), B: mm(x2, y2), Width: int64(wMM * 1e6), Layer: layer}
	}
	via := func(x, y, sizeMM, drillMM float64) *geom.Via {
		return &geom.Via{At: mm(x, y), Size: int64(sizeMM * 1e6), Drill: int64(drillMM * 1e6)}
	}
	return &geom.BoardGeometry{UnitNm: 1, Nets: []*geom.NetCopper{
		{Net: "THIN", Segments: []*geom.TrackSegment{seg(10, 10, 14, 10, 0.05, "F.Cu")}},
		{Net: "CLOSE_A", Segments: []*geom.TrackSegment{seg(10, 12, 14, 12, 0.15, "F.Cu")}},
		{Net: "CLOSE_B", Segments: []*geom.TrackSegment{
			seg(10, 12.19, 14, 12.19, 0.15, "F.Cu"), // 0.04mm edge gap to CLOSE_A -> fires
			seg(10, 12.19, 14, 12.19, 0.15, "B.Cu"), // same gap, other layer -> silent
		}},
		{Net: "SAMENET", Segments: []*geom.TrackSegment{ // close to itself -> silent
			seg(20, 20, 24, 20, 0.15, "F.Cu"),
			seg(20, 20.17, 24, 20.17, 0.15, "F.Cu"),
		}},
		{Net: "SMALLHOLE", Vias: []*geom.Via{via(30, 30, 0.4, 0.1)}},
		{Net: "THINRING", Vias: []*geom.Via{via(32, 30, 0.5, 0.4)}},
		{Net: "CLEAN", Segments: []*geom.TrackSegment{seg(40, 40, 44, 40, 0.25, "F.Cu")},
			Vias: []*geom.Via{via(40, 42, 0.8, 0.4)}},
	}}
}

func drcFindings(t *testing.T) map[string][]string {
	t.Helper()
	m := check.NewModelWithBoard(&ir.Design{}, drcBoard())
	got := map[string][]string{}
	for _, r := range []*check.Rule{trackWidth, holeSize, annularWidth, copperClearance} {
		for _, f := range r.Findings(m) {
			if f.Subject.Kind != check.KindNet {
				t.Errorf("%s: finding kind = %q, want net", r.Name, f.Subject.Kind)
			}
			got[r.Name] = append(got[r.Name], check.EntityRef(f.Subject))
		}
	}
	return got
}

func TestBoardDRCRules(t *testing.T) {
	got := drcFindings(t)
	want := map[string][]string{
		"track-width":      {"THIN"},
		"hole-size":        {"SMALLHOLE"},
		"annular-width":    {"THINRING"},
		"copper-clearance": {"CLOSE_A"},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("findings = %v, want %v", got, want)
	}
}

func TestCopperClearanceMessageNamesBothNets(t *testing.T) {
	m := check.NewModelWithBoard(&ir.Design{}, drcBoard())
	fs := copperClearance.Findings(m)
	if len(fs) != 1 {
		t.Fatalf("findings = %+v", fs)
	}
	msg := fs[0].Message
	if !strings.Contains(msg, `"CLOSE_A"`) || !strings.Contains(msg, `"CLOSE_B"`) || !strings.Contains(msg, "worst gap 0.040mm") {
		t.Errorf("message = %q", msg)
	}
}

// TestBoardRulesSilentWithoutBoard: the same rules over a netlist-only model produce
// nothing — geometric rules never guess.
func TestBoardRulesSilentWithoutBoard(t *testing.T) {
	m := check.NewModel(ruleFixture())
	for _, r := range []*check.Rule{trackWidth, holeSize, annularWidth, copperClearance} {
		if fs := r.Findings(m); len(fs) != 0 {
			t.Errorf("%s fired %d finding(s) with no board tier", r.Name, len(fs))
		}
	}
}

// TestBoardRuleAvailability: the board. read prefix gates like param(...): unavailable
// for a design whose source carries no board geometry, available for a board read and
// for the design-less catalog listing.
func TestBoardRuleAvailability(t *testing.T) {
	if ok, _ := check.Available(trackWidth, check.NewModel(&ir.Design{SourceFormat: "edif"})); ok {
		t.Error("board rule available on a netlist-only design")
	}
	if ok, reason := check.Available(trackWidth, check.NewModel(&ir.Design{SourceFormat: "kicad-pcb"})); !ok {
		t.Errorf("board rule unavailable on a board design: %s", reason)
	}
	if ok, reason := check.Available(trackWidth, check.NewModel(&ir.Design{SourceFormat: "ipc-2581"})); !ok {
		t.Errorf("board rule unavailable on an IPC-2581 board design: %s", reason)
	}
	if ok, _ := check.Available(trackWidth, nil); !ok {
		t.Error("board rule unavailable in the catalog listing (nil design)")
	}
	// WS3-089: a netlist design (edif) with a SEPARATE board tier attached (the agni review
	// --board-path path) ungates the geometric rules — the gate follows the attached tier, not
	// the source format, which stays edif on the netlist entry.
	if ok, reason := check.Available(trackWidth, check.NewModelWithBoard(&ir.Design{SourceFormat: "edif"}, drcBoard())); !ok {
		t.Errorf("board rule unavailable on a netlist design with a board tier attached: %s", reason)
	}
}
