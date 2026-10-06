package builtin

import (
	"context"
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

// drcBoard places every violation class once, on its own net, plus clean copper that must not
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
	m := check.NewModel(&ir.Design{}, check.WithBoard(drcBoard()))
	got := map[string][]string{}
	for _, r := range []*check.Rule{trackWidth, holeSize, annularWidth, copperClearance} {
		for _, f := range r.Findings(context.Background(), m) {
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
	m := check.NewModel(&ir.Design{}, check.WithBoard(drcBoard()))
	fs := copperClearance.Findings(context.Background(), m)
	if len(fs) != 1 {
		t.Fatalf("findings = %+v", fs)
	}
	msg := fs[0].Message
	if !strings.Contains(msg, `"CLOSE_A"`) || !strings.Contains(msg, `"CLOSE_B"`) || !strings.Contains(msg, "worst gap 0.040mm") {
		t.Errorf("message = %q", msg)
	}
}

// TestBoardRulesSilentWithoutBoard checks that the same rules over a netlist-only model produce
// nothing, because geometric rules never guess.
func TestBoardRulesSilentWithoutBoard(t *testing.T) {
	m := check.NewModel(ruleFixture())
	for _, r := range []*check.Rule{trackWidth, holeSize, annularWidth, copperClearance} {
		if fs := r.Findings(context.Background(), m); len(fs) != 0 {
			t.Errorf("%s fired %d finding(s) with no board tier", r.Name, len(fs))
		}
	}
}

// TestBoardRuleAvailability checks that the board. read prefix gates like param.max(...). It is unavailable
// for a design whose source carries no board geometry, and available for a board read and
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
	// --board-path path) ungates the geometric rules. The gate follows the attached tier, not
	// the source format, which stays edif on the netlist entry.
	if ok, reason := check.Available(trackWidth, check.NewModel(&ir.Design{SourceFormat: "edif"}, check.WithBoard(drcBoard()))); !ok {
		t.Errorf("board rule unavailable on a netlist design with a board tier attached: %s", reason)
	}
}

// declaredRules is a design carrying a board_rules constraint, the minimums a KiCad project
// declares in board.design_settings.rules (agni issue 933).
func declaredRules(track, clearance, drill, annular string) *ir.Design {
	return &ir.Design{Constraints: []*ir.Constraint{{Name: "board", Kind: check.BoardRulesConstraintKind, Params: map[string]string{
		"min_track_width": track, "min_clearance": clearance, "min_through_hole_diameter": drill, "min_via_annular_width": annular,
	}}}}
}

// A board routed for a finer process than the default floors passes against its own declaration.
// Every violation drcBoard places sits above these, so nothing fires.
func TestBoardDRCRulesUseTheBoardsDeclaredMinimums(t *testing.T) {
	m := check.NewModel(declaredRules("0.04", "0.03", "0.05", "0.04"), check.WithBoard(drcBoard()))
	for _, r := range []*check.Rule{trackWidth, holeSize, annularWidth, copperClearance} {
		if fs := r.Findings(context.Background(), m); len(fs) != 0 {
			t.Errorf("%s fired against a board that declares a finer minimum: %v", r.Name, fs)
		}
	}
}

// A declaration coarser than the copper fails it, even copper that clears the default floor, and the
// finding names the declared minimum rather than the default one.
func TestBoardDRCRulesFailCopperBelowTheBoardsOwnMinimum(t *testing.T) {
	m := check.NewModel(declaredRules("0.3", "0.127", "0.5", "0.075"), check.WithBoard(drcBoard()))
	got := map[string]string{}
	for _, r := range []*check.Rule{trackWidth, holeSize} {
		for _, f := range r.Findings(context.Background(), m) {
			if check.EntityRef(f.Subject) == "CLEAN" {
				got[r.Name] = f.Message
			}
		}
	}
	if !strings.Contains(got["track-width"], "0.3mm minimum the board declares") {
		t.Errorf("track-width on CLEAN = %q, want it below the declared 0.3mm", got["track-width"])
	}
	if !strings.Contains(got["hole-size"], "0.5mm minimum the board declares") {
		t.Errorf("hole-size on CLEAN = %q, want it below the declared 0.5mm", got["hole-size"])
	}
}

// A declaration that is missing, zero or not a number leaves the default floor in force.
func TestBoardDRCRulesFallBackToTheFloorWithoutADeclaration(t *testing.T) {
	m := check.NewModel(declaredRules("", "0", "abc", ""), check.WithBoard(drcBoard()))
	fs := trackWidth.Findings(context.Background(), m)
	if len(fs) != 1 || !strings.Contains(fs[0].Message, "0.127mm fabrication floor") {
		t.Errorf("track-width = %v, want THIN against the 0.127mm fabrication floor", fs)
	}
	if cc := copperClearance.Findings(context.Background(), m); len(cc) != 1 || !strings.Contains(cc[0].Message, "0.127mm fabrication floor") {
		t.Errorf("copper-clearance = %v, want CLOSE_A against the default floor", cc)
	}
}

// Copper routed at exactly the minimum measures a nanometre or two under it between diagonal
// tracks, and passes as it does in KiCad's DRC; a gap a micron under the minimum still fails.
func TestCopperClearanceAllowsKiCadsDRCEpsilon(t *testing.T) {
	pair := func(gapNm int64) *geom.BoardGeometry {
		w := int64(150_000)
		y := 12_000_000 + w + gapNm // centre-to-centre is one width plus the edge gap
		return &geom.BoardGeometry{UnitNm: 1, Nets: []*geom.NetCopper{
			{Net: "A", Segments: []*geom.TrackSegment{{A: &geom.Point{X: 10_000_000, Y: 12_000_000}, B: &geom.Point{X: 14_000_000, Y: 12_000_000}, Width: w, Layer: "F.Cu"}}},
			{Net: "B", Segments: []*geom.TrackSegment{{A: &geom.Point{X: 10_000_000, Y: y}, B: &geom.Point{X: 14_000_000, Y: y}, Width: w, Layer: "F.Cu"}}},
		}}
	}
	d := declaredRules("", "0.1", "", "")
	if fs := copperClearance.Findings(context.Background(), check.NewModel(d, check.WithBoard(pair(99_999)))); len(fs) != 0 {
		t.Errorf("a gap 1nm under the declared 0.1mm fired: %v", fs)
	}
	if fs := copperClearance.Findings(context.Background(), check.NewModel(d, check.WithBoard(pair(99_000)))); len(fs) != 1 {
		t.Errorf("a gap 1um under the declared 0.1mm must fail, got %v", fs)
	}
}
