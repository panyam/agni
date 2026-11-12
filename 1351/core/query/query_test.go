package query

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/core/facts"
	"github.com/panyam/agni/core/param"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/jaala/ns"
)

// boardGeom is a two-net board, with SIG routed thin (0.08mm) and PWR wide (0.5mm).
func boardGeom() *geom.BoardGeometry {
	return &geom.BoardGeometry{UnitNm: 1, Nets: []*geom.NetCopper{
		{Net: "SIG", Segments: []*geom.TrackSegment{{Width: 80000, Layer: "F.Cu"}}},
		{Net: "PWR", Segments: []*geom.TrackSegment{{Width: 500000, Layer: "F.Cu"}}},
	}}
}

// TestBoardQuery (WS1-041) checks that a board relation is queryable through the same engine with
// no evaluator change. The query asks for nets routed thinner than 0.1mm.
func TestBoardQuery(t *testing.T) {
	rows := runQuery(t, check.NewModel(&ir.Design{}, check.WithBoard(boardGeom())), `board.track_width(?net,?w), ?w < 0.1mm => ?net, ?w`)
	if len(rows) != 1 || rows[0].Bind["net"].S != "SIG" {
		t.Errorf("rows = %+v, want SIG (0.08mm thin)", rows)
	}
	// Shown as the board's own tools print it, in millimetres, whatever the engine folds it to.
	if len(rows) == 1 && Display(rows[0].Bind["w"]) != "0.08" {
		t.Errorf("width shows as %q, want 0.08", Display(rows[0].Bind["w"]))
	}
}

// A length is published in metres, so a BARE number compared with one is read as metres. So 0.1
// means a 100 mm track, and both nets are thinner than that (agni issue 1035). A threshold over a
// length writes its unit, as TestBoardQuery's does.
func TestABareThresholdOverALengthIsMetres(t *testing.T) {
	rows := runQuery(t, check.NewModel(&ir.Design{}, check.WithBoard(boardGeom())), `board.track_width(?net,?w), ?w < 0.1 => ?net`)
	if len(rows) != 2 {
		t.Errorf("rows = %+v, want both nets, since 0.1 m is wider than either", rows)
	}
	rows = runQuery(t, check.NewModel(&ir.Design{}, check.WithBoard(boardGeom())), `board.track_width(?net,?w), ?w < 0.0001 => ?net`)
	if len(rows) != 1 || rows[0].Bind["net"].S != "SIG" {
		t.Errorf("rows = %+v, want SIG alone, since 0.0001 m is 0.1 mm", rows)
	}
}

// TestCrossTierJoin (WS1-041) runs one query that joins BOARD ⋈ NETLIST ⋈ DATASHEET. It finds a
// thin-routed net, the part on it, and the part's datasheet identity, and the answer cites the
// board and the schematic. Tier-generality exists for this case, since the query spans copper,
// connectivity, and datasheets at once.
func TestCrossTierJoin(t *testing.T) {
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "U1", Mpn: "REG-24", Prov: &ir.Provenance{SourceFile: "x"}}},
		Nets:       []*ir.Net{{Name: "SIG", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "x"}}},
	}
	m := check.NewModel(d, check.WithBoard(boardGeom()), check.WithParamProvider(param.ParamSet{"REG-24": regSpec("REG-24", 20)}))
	rows := runQuery(t, m, `board.track_width(?net,?w), component.net(?ref,?net), component.mpn(?ref,?mpn), ?w < 0.1mm => ?net, ?ref, ?mpn`)
	if len(rows) != 1 || rows[0].Bind["ref"].S != "U1" || rows[0].Bind["mpn"].S != "REG-24" || rows[0].Bind["net"].S != "SIG" {
		t.Fatalf("cross-tier rows = %+v, want SIG/U1/REG-24", rows)
	}
	joined := strings.Join(rows[0].Cites, " | ")
	if !strings.Contains(joined, "board net SIG") || !strings.Contains(joined, "x") {
		t.Errorf("cites %q should span board (board net SIG) and netlist (x)", joined)
	}
}

func f64(x float64) *float64 { return &x }

// regSpec is a seeded part with one absolute-maximum VIN row, cited to a page. It is the datasheet
// side of the showcase join.
func regSpec(mpn string, vinMax float64) *parampb.PartSpec {
	return &parampb.PartSpec{
		Mpn: mpn, Manufacturer: "Acme",
		Docs: []*parampb.SourceDoc{{Id: "ds", Title: "REG-DS Rev A"}},
		Parameters: []*parampb.Parameter{{
			Name: "Supply voltage", Symbol: "VIN",
			LimitKind:         parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX,
			Value:             &parampb.RangeValue{Max: f64(vinMax)},
			Unit:              "V",
			ConditionCoverage: parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE,
			Prov:              &parampb.ParamProvenance{DocRef: "ds", Page: 4, TableOrFigure: "Abs Max", Method: "hand", Confidence: 1},
		}},
	}
}

// regDesign places a regulator U1 (MPN REG-24) with pin 1 on railNet.
func regDesign(railNet string) *ir.Design {
	return &ir.Design{
		Components: []*ir.Component{{RefDes: "U1", Mpn: "REG-24", Prov: &ir.Provenance{SourceFile: "reg"}}},
		Nets:       []*ir.Net{{Name: railNet, Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "reg"}}},
	}
}

func runQuery(t *testing.T, m check.Model, text string) []Row {
	t.Helper()
	q, err := Parse(text)
	if err != nil {
		t.Fatalf("Parse(%q): %v", text, err)
	}
	rows, err := (Naive{}).Eval(context.Background(), q, NewBase(m))
	if err != nil {
		t.Fatalf("Eval(%q): %v", text, err)
	}
	return rows
}

// runQueryOn is runQuery over an explicit relation vocabulary, for a test that composes its own
// rather than adding to the process default.
func runQueryOn(t *testing.T, reg *facts.Registry, m check.Model, text string) []Row {
	t.Helper()
	q, err := Parse(text)
	if err != nil {
		t.Fatalf("Parse(%q): %v", text, err)
	}
	rows, err := (Naive{}).Eval(context.Background(), q, NewBaseFrom(reg, m))
	if err != nil {
		t.Fatalf("Eval(%q): %v", text, err)
	}
	return rows
}

// TestShowcaseJoin (WS3-029) runs the flagship datasheet query. It asks for a part whose abs-max
// VIN is below the rail it sits on. The query joins param.max ⋈ component.mpn ⋈ component.net ⋈
// net.max_voltage, and the answer carries provenance.
func TestShowcaseJoin(t *testing.T) {
	m := check.NewModel(regDesign("+24V"), check.WithParamProvider(param.ParamSet{"REG-24": regSpec("REG-24", 20)}))
	rows := runQuery(t, m,
		`component.mpn(?ref,?mpn), param.max(?mpn,"VIN",?vmax), component.net(?ref,?net), net.max_voltage(?net,?rail), ?vmax < ?rail => ?ref, ?vmax, ?net, ?rail`)

	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1 (U1 over-stressed on +24V)", len(rows))
	}
	r := rows[0]
	if r.Bind["ref"].S != "U1" || r.Bind["net"].S != "+24V" {
		t.Errorf("row = %+v, want ref=U1 net=+24V", r.Bind)
	}
	if r.Bind["vmax"].Num == nil || *r.Bind["vmax"].Num != 20 || r.Bind["rail"].Num == nil || *r.Bind["rail"].Num != 24 {
		t.Errorf("vmax/rail = %v/%v, want 20/24", r.Bind["vmax"], r.Bind["rail"])
	}
	// Verifiability: the answer cites the datasheet page and the schematic source.
	joined := strings.Join(r.Cites, " | ")
	for _, want := range []string{"REG-DS Rev A", "page 4", "reg"} {
		if !strings.Contains(joined, want) {
			t.Errorf("cites %q missing %q", joined, want)
		}
	}
}

// TestShowcasePasses (WS3-029) checks that the same query is silent when the rail is within the
// abs-max. The comparison prunes, so no answer is an answer.
func TestShowcasePasses(t *testing.T) {
	m := check.NewModel(regDesign("+12V"), check.WithParamProvider(param.ParamSet{"REG-24": regSpec("REG-24", 20)}))
	rows := runQuery(t, m,
		`component.mpn(?ref,?mpn), param.max(?mpn,"VIN",?vmax), component.net(?ref,?net), net.max_voltage(?net,?rail), ?vmax < ?rail => ?ref`)
	if len(rows) != 0 {
		t.Errorf("rows = %v, want none (20V abs-max, 12V rail)", rows)
	}
}

// vddSpec is REG-24 with TWO rows on the ONE symbol VDD. One is a recommended-operating window
// (3.0..3.6), and the other is an absolute-maximum ceiling (4.6). The thin
// param.max(mpn,symbol,max) relation cannot tell them apart, since both surface as
// param.max(REG-24,"VDD",...). param.range separates them by its kind argument.
func vddSpec(mpn string) *parampb.PartSpec {
	return &parampb.PartSpec{
		Mpn: mpn, Manufacturer: "Acme",
		Docs: []*parampb.SourceDoc{{Id: "ds", Title: "REG-DS Rev A"}},
		Parameters: []*parampb.Parameter{
			{
				Name: "Supply, recommended", Symbol: "VDD",
				LimitKind: parampb.LimitKind_LIMIT_KIND_RECOMMENDED_OPERATING,
				Value:     &parampb.RangeValue{Min: f64(3.0), Max: f64(3.6)}, Unit: "V",
				Prov: &parampb.ParamProvenance{DocRef: "ds", Page: 6, TableOrFigure: "Recommended Operating", Method: "hand", Confidence: 1},
			},
			{
				Name: "Supply, absolute max", Symbol: "VDD",
				LimitKind: parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX,
				Value:     &parampb.RangeValue{Max: f64(4.6)}, Unit: "V",
				Prov: &parampb.ParamProvenance{DocRef: "ds", Page: 4, TableOrFigure: "Abs Max", Method: "hand", Confidence: 1},
			},
		},
	}
}

// TestParamRangeTwoSidedJoin (WS3-082) shows that datalog can express a two-sided range check that
// discriminates by limit kind, using param.range + net.nominal_voltage. The thin
// param.max(mpn,symbol,max) relation could not express that join. It has no lower bound, and no way
// to tell an absolute-max row from a recommended-operating one on the same symbol. This test is the
// ticket's "demonstrated datalog program". It proves the enriched vocabulary is sufficient for the
// datasheet-range rule family.
func TestParamRangeTwoSidedJoin(t *testing.T) {
	m := check.NewModel(regDesign("+5V"), check.WithParamProvider(param.ParamSet{"REG-24": vddSpec("REG-24")}))

	// Over the recommended maximum: +5V > 3.6. The "recommended_operating" kind filter keeps the
	// abs-max row (4.6) out of this check, so exactly one answer.
	over := runQuery(t, m,
		`component.mpn(?ref,?mpn), param.range(?mpn,?sym,"recommended_operating",?min,?max), component.net(?ref,?net), net.nominal_voltage(?net,?v), ?v > ?max => ?ref, ?net, ?v, ?max`)
	if len(over) != 1 {
		t.Fatalf("recommended over-max: rows = %d, want 1", len(over))
	}
	r := over[0]
	if r.Bind["ref"].S != "U1" || r.Bind["net"].S != "+5V" {
		t.Errorf("row = %+v, want ref=U1 net=+5V", r.Bind)
	}
	if r.Bind["v"].Num == nil || *r.Bind["v"].Num != 5 || r.Bind["max"].Num == nil || *r.Bind["max"].Num != 3.6 {
		t.Errorf("v/max = %v/%v, want 5/3.6", r.Bind["v"], r.Bind["max"])
	}

	// Under the recommended MINIMUM. The thin param relation has no bound on this side. A +2V5 rail
	// on the same part is below the 3.0 floor.
	under := runQuery(t, check.NewModel(regDesign("+2V5"), check.WithParamProvider(param.ParamSet{"REG-24": vddSpec("REG-24")})),
		`component.mpn(?ref,?mpn), param.range(?mpn,?sym,"recommended_operating",?min,?max), component.net(?ref,?net), net.nominal_voltage(?net,?v), ?v < ?min => ?ref, ?v, ?min`)
	if len(under) != 1 || under[0].Bind["min"].Num == nil || *under[0].Bind["min"].Num != 3.0 {
		t.Fatalf("recommended under-min: rows = %v, want one with min 3.0", under)
	}

	// Kind discrimination. Filtering the SAME symbol by kind picks the abs-max ceiling (4.6).
	// param.max(mpn,"VDD",max) could never separate that ceiling from the recommended 3.6.
	abs := runQuery(t, m, `param.range("REG-24","VDD","absolute_max",?min,?max) => ?max`)
	if len(abs) != 1 || abs[0].Bind["max"].Num == nil || *abs[0].Bind["max"].Num != 4.6 {
		t.Fatalf("abs-max row: rows = %v, want one with max 4.6", abs)
	}
}

// TestSingleRelation (WS3-029) checks that a one-atom query is a plain select-project over the fact
// base.
func TestSingleRelation(t *testing.T) {
	m := check.NewModel(regDesign("+24V"), check.WithParamProvider(param.ParamSet{"REG-24": regSpec("REG-24", 20)}))
	rows := runQuery(t, m, `component.mpn(?ref,?mpn) => ?ref, ?mpn`)
	if len(rows) != 1 || rows[0].Bind["ref"].S != "U1" || rows[0].Bind["mpn"].S != "REG-24" {
		t.Errorf("rows = %+v, want one (U1, REG-24)", rows)
	}
}

// reachDesign bridges nets A and B with a series resistor R1 (a pass element the reach walk crosses).
func reachDesign() *ir.Design {
	return &ir.Design{
		Components: []*ir.Component{{RefDes: "R1", Sections: []*ir.ComponentSection{{PartRef: "R"}}, Prov: &ir.Provenance{SourceFile: "r"}}},
		Nets: []*ir.Net{
			{Name: "A", Connections: []*ir.Connection{{ComponentRef: "R1", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "r"}},
			{Name: "B", Connections: []*ir.Connection{{ComponentRef: "R1", PinRef: "2"}}, Prov: &ir.Provenance{SourceFile: "r"}},
		},
	}
}

// TestReachesRecursion (WS3-029) checks that the built-in reaches relation makes recursion real.
// The relation is bridged to check.Model.Reach. reaches from A crosses the series resistor to B,
// and it is reflexive, so A too.
func TestReachesRecursion(t *testing.T) {
	rows := runQuery(t, check.NewModel(reachDesign()), `net.reaches("A",?n) => ?n`)
	got := map[string]bool{}
	for _, r := range rows {
		got[r.Bind["n"].S] = true
	}
	if !got["A"] || !got["B"] {
		t.Errorf("net.reaches(A) = %v, want A and B (through the series resistor)", got)
	}
}

// reachChainDesign is a straight series chain N0 -R1- N1 -R2- N2 -R3- N3, so every net sits at a
// known, distinct distance from N0. If everything were one hop away, the fixture could not tell a
// radius filter from a no-op, so the graded distance matters.
func reachChainDesign() *ir.Design {
	d := &ir.Design{}
	for i := 1; i <= 3; i++ {
		d.Components = append(d.Components, &ir.Component{
			RefDes: fmt.Sprintf("R%d", i), Sections: []*ir.ComponentSection{{PartRef: "R"}},
			Prov: &ir.Provenance{SourceFile: "c"},
		})
	}
	for i := 0; i <= 3; i++ {
		var conns []*ir.Connection
		if i > 0 {
			conns = append(conns, &ir.Connection{ComponentRef: fmt.Sprintf("R%d", i), PinRef: "2"})
		}
		if i < 3 {
			conns = append(conns, &ir.Connection{ComponentRef: fmt.Sprintf("R%d", i+1), PinRef: "1"})
		}
		d.Nets = append(d.Nets, &ir.Net{
			Name: fmt.Sprintf("N%d", i), Connections: conns, Prov: &ir.Provenance{SourceFile: "c"},
		})
	}
	return d
}

// TestReachesBindsDistance (WS3-112) checks that the optional third argument binds the number of
// series crossings, reflexive at 0. So a rule states its own radius instead of inheriting the
// engine's.
func TestReachesBindsDistance(t *testing.T) {
	rows := runQuery(t, check.NewModel(reachChainDesign()), `net.reaches("N0", ?n, ?h) => ?n, ?h`)
	got := map[string]string{}
	for _, r := range rows {
		got[r.Bind["n"].S] = r.Bind["h"].S
	}
	want := map[string]string{"N0": "0", "N1": "1", "N2": "2", "N3": "3"}
	for n, w := range want {
		if got[n] != w {
			t.Errorf("net.reaches(N0, %s, ?h) bound %q, want %q (full: %v)", n, got[n], w, got)
		}
	}
}

// TestReachesRadiusFilter (WS3-112) would have caught a 2-hop protection predicate composed out of
// the 100-hop reaches built-in. A clamp three series elements away must NOT satisfy a "within two
// hops" question. Reporting that it does is a false PASS on a real defect.
func TestReachesRadiusFilter(t *testing.T) {
	rows := runQuery(t, check.NewModel(reachChainDesign()), `net.reaches("N0", ?n, ?h), ?h <= 2 => ?n`)
	got := map[string]bool{}
	for _, r := range rows {
		got[r.Bind["n"].S] = true
	}
	if !got["N0"] || !got["N1"] || !got["N2"] {
		t.Errorf("within 2 hops = %v, want N0, N1 and N2", got)
	}
	if got["N3"] {
		t.Errorf("within 2 hops included N3, which is 3 series elements away: %v", got)
	}
}

// TestReachesConstantHopsIsExact (WS3-112) pins a semantics that the doc and catalog warn about. A
// bare constant in the third slot binds by equality, so it means EXACTLY that many hops. A reader
// who writes it expecting "within" gets a silently narrower answer. So the radius idiom is a
// comparison.
func TestReachesConstantHopsIsExact(t *testing.T) {
	rows := runQuery(t, check.NewModel(reachChainDesign()), `net.reaches("N0", ?n, 2) => ?n`)
	got := map[string]bool{}
	for _, r := range rows {
		got[r.Bind["n"].S] = true
	}
	if len(got) != 1 || !got["N2"] {
		t.Errorf("net.reaches(N0, ?n, 2) = %v, want exactly {N2} (equality, not a bound)", got)
	}
}

// TestReachesArityBothPathsAgree (WS3-112) checks that the POSITIVE path and the NEGATION path
// admit the optional argument through one predicate. If they diverged, the engine would accept
// net.reaches(?a,?b,?h) while rejecting `not net.reaches(?a,?b,?h)`. Negation is validated up
// front, so the whole query would fail rather than degrade.
func TestReachesArityBothPathsAgree(t *testing.T) {
	m := check.NewModel(reachChainDesign())
	rows := runQuery(t, m, `component.net(?r, ?n), not net.reaches("N0", ?n, ?h) => ?n`)
	for _, r := range rows {
		if n := r.Bind["n"].S; n == "N1" {
			t.Errorf("not net.reaches(N0, N1, ?h) should not hold: %v", rows)
		}
	}
	q, err := Parse(`net.reaches(?a, ?b, ?c, ?d) => ?a`)
	if err == nil {
		if _, err = (Naive{}).Eval(context.Background(), q, NewBase(m)); err == nil {
			t.Error("reaches at arity 4: want an error naming the accepted arity")
		}
	}
}

// twoPartDesign places U1 (has a VIN spec) and U2 (no spec) on separate rails.
func twoPartDesign() (*ir.Design, param.ParamSet) {
	d := &ir.Design{
		Components: []*ir.Component{
			{RefDes: "U1", Mpn: "REG-24", Prov: &ir.Provenance{SourceFile: "d"}},
			{RefDes: "U2", Mpn: "PLAIN", Prov: &ir.Provenance{SourceFile: "d"}},
		},
		Nets: []*ir.Net{
			{Name: "+24V", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "d"}},
			{Name: "+5V", Connections: []*ir.Connection{{ComponentRef: "U2", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "d"}},
		},
	}
	// Only REG-24 is seeded (PLAIN has no spec -> no param facts), so U2's MPN has no VIN param.
	return d, param.ParamSet{"REG-24": regSpec("REG-24", 20)}
}

// TestNegation (WS3-029 fast-follow) checks that `not param.max(?m,"VIN",?v)` keeps the mpns with
// NO VIN param. ?v appears only under negation, so it is an existential wildcard ("no VIN param for
// any value"). The positive literal binds ?m, and ?m must match.
func TestNegation(t *testing.T) {
	d, set := twoPartDesign()
	m := check.NewModel(d, check.WithParamProvider(set))
	rows := runQuery(t, m, `component.mpn(?r,?m), not param.max(?m,"VIN",?v) => ?m`)
	if len(rows) != 1 || rows[0].Bind["m"].S != "PLAIN" {
		t.Errorf("rows = %+v, want only PLAIN (REG-24 has a VIN param, so it is excluded)", rows)
	}
	if len(rows) == 1 && len(rows[0].Cites) == 0 {
		t.Error("a negation answer still carries the positive facts' provenance")
	}
}

// TestAggregationCount (WS3-029 fast-follow) checks that count(?ref) grouped by ?net gives
// parts-per-net.
func TestAggregationCount(t *testing.T) {
	// Both U1 and U2 sit on SHARED, plus their own rails.
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "d"}}, {RefDes: "U2", Prov: &ir.Provenance{SourceFile: "d"}}},
		Nets: []*ir.Net{
			{Name: "SHARED", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}, {ComponentRef: "U2", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "d"}},
			{Name: "LONE", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "2"}}, Prov: &ir.Provenance{SourceFile: "d"}},
		},
	}
	rows := runQuery(t, check.NewModel(d), `component.net(?r,?n) => ?n, count(?r)`)
	got := map[string]string{}
	for _, r := range rows {
		got[r.Bind["n"].S] = r.Bind["count(r)"].S
	}
	if got["SHARED"] != "2" || got["LONE"] != "1" {
		t.Errorf("counts = %v, want SHARED=2 LONE=1", got)
	}
}

// TestAggregationMax (WS3-029 fast-follow) checks that max over a numeric column reduces per group.
//
// Both rows are VOLTS. The fixture used to pair a 20 V row with an 800 mA one and assert 800. That
// assertion depended on 800 being the larger PRINTED number across two different units. Such a
// comparison has no meaning. Since agni issue 165 reduced the query surface's numbers to SI base
// units, 800 is not even the larger number (800 mA is 0.8 A). The property under test is that max()
// reduces per group, so the fixture states two quantities that can actually be ordered.
func TestAggregationMax(t *testing.T) {
	spec := regSpec("REG-24", 20)
	// add a second numeric param so max has something to choose
	spec.Parameters = append(spec.Parameters, &parampb.Parameter{
		Name: "ESD tolerance", Symbol: "VESD", LimitKind: parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX,
		Value: &parampb.RangeValue{Max: f64(800)}, Unit: "V",
		Prov: &parampb.ParamProvenance{DocRef: "ds", Page: 4, Method: "hand", Confidence: 1},
	})
	m := check.NewModel(regDesign("+24V"), check.WithParamProvider(param.ParamSet{"REG-24": spec}))
	rows := runQuery(t, m, `param.max(?mpn,?sym,?max) => ?mpn, max(?max)`)
	if len(rows) != 1 || rows[0].Bind["max(max)"].S != "800" {
		t.Errorf("rows = %+v, want one with max=800", rows)
	}
}

// TestStringPredicates (WS3-029 fast-follow) checks that contains/prefix/suffix filter a bound
// string. Both positive and negated forms work, so a search box can ask "MPN contains LM" or "not".
func TestStringPredicates(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "U1", Mpn: "LM1117", Prov: &ir.Provenance{SourceFile: "d"}},
		{RefDes: "U2", Mpn: "REG-24", Prov: &ir.Provenance{SourceFile: "d"}},
	}}
	m := check.NewModel(d, check.WithParamProvider(param.ParamSet{"LM1117": regSpec("LM1117", 20), "REG-24": regSpec("REG-24", 20)}))

	one := func(text, wantRef string) {
		t.Helper()
		rows := runQuery(t, m, text)
		if len(rows) != 1 || rows[0].Bind["r"].S != wantRef {
			t.Errorf("%s => %+v, want only %s", text, rows, wantRef)
		}
	}
	one(`component.mpn(?r,?m), str.contains(?m,"LM") => ?r`, "U1")
	one(`component.mpn(?r,?m), str.prefix(?m,"REG") => ?r`, "U2")
	one(`component.mpn(?r,?m), str.suffix(?m,"1117") => ?r`, "U1")
	one(`component.mpn(?r,?m), not str.contains(?m,"LM") => ?r`, "U2") // negated string predicate
}

// TestStringPredicateUnbound (WS3-029 fast-follow) checks a string predicate whose value no
// relation binds. It errors clearly, because it filters and cannot enumerate every string.
func TestStringPredicateUnbound(t *testing.T) {
	if _, err := (Naive{}).Eval(context.Background(), mustParse(t, `str.contains(?x,"LM") => ?x`), NewBase(check.NewModel(&ir.Design{}))); err == nil {
		t.Error("contains on an unbound variable succeeded; want an error")
	}
}

func mustParse(t *testing.T, s string) Query {
	t.Helper()
	q, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return q
}

// chainDesign wires U1-U2 on net N1 and U2-U3 on net N2. U1 and U3 share no net, so they are linked
// only transitively (through U2). The recursion tests use it, since a transitive closure reaches U3
// from U1 and a single join cannot.
func chainDesign() *ir.Design {
	return &ir.Design{
		Components: []*ir.Component{
			{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "c"}},
			{RefDes: "U2", Prov: &ir.Provenance{SourceFile: "c"}},
			{RefDes: "U3", Prov: &ir.Provenance{SourceFile: "c"}},
		},
		Nets: []*ir.Net{
			{Name: "N1", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}, {ComponentRef: "U2", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "c"}},
			{Name: "N2", Connections: []*ir.Connection{{ComponentRef: "U2", PinRef: "2"}, {ComponentRef: "U3", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "c"}},
		},
	}
}

// TestUserRuleView (WS3-029 fast-follow) checks that a non-recursive user rule is a named view.
// `sharesnet` derives the pairs of distinct components on a common net, and the goal queries that
// IDB relation.
func TestUserRuleView(t *testing.T) {
	rows := runQuery(t, check.NewModel(chainDesign()),
		`sharesnet(?a,?b) :- component.net(?a,?n), component.net(?b,?n), ?a != ?b; sharesnet("U1",?x) => ?x`)
	got := map[string]bool{}
	for _, r := range rows {
		got[r.Bind["x"].S] = true
	}
	if len(got) != 1 || !got["U2"] {
		t.Errorf("sharesnet(U1) = %v, want only U2 (U1 shares N1 with U2, nothing else directly)", got)
	}
}

// TestUserRuleRecursion (WS3-029 fast-follow) is the flagship, a user-defined transitive closure.
// `connected` is its own body atom, so it runs to fixpoint and reaches U3 from U1 through U2. No
// single join can do that. The derived answer still carries the base nets' provenance.
func TestUserRuleRecursion(t *testing.T) {
	q := `link(?a,?b) :- component.net(?a,?n), component.net(?b,?n), ?a != ?b;
	      connected(?a,?b) :- link(?a,?b);
	      connected(?a,?c) :- connected(?a,?b), link(?b,?c);
	      connected("U1",?x) => ?x`
	rows := runQuery(t, check.NewModel(chainDesign()), q)
	got := map[string]bool{}
	for _, r := range rows {
		got[r.Bind["x"].S] = true
	}
	if !got["U2"] || !got["U3"] {
		t.Errorf("connected(U1) = %v, want U2 (direct) and U3 (transitive, through U2)", got)
	}
	if len(rows) > 0 && len(rows[0].Cites) == 0 {
		t.Error("a derived (recursive) answer still carries base-fact provenance")
	}
}

// TestStratifiedNegationInRule (WS3-029 fast-follow) checks that a rule may negate another IDB
// relation, and stratification derives that relation fully first. `linked` collects components that
// share a net with someone, and `isolated` is a component that is NOT linked. U_LONE sits on its
// own net, so it is the only isolated part.
func TestStratifiedNegationInRule(t *testing.T) {
	d := chainDesign()
	d.Components = append(d.Components, &ir.Component{RefDes: "U_LONE", Prov: &ir.Provenance{SourceFile: "c"}})
	d.Nets = append(d.Nets, &ir.Net{Name: "N3", Connections: []*ir.Connection{{ComponentRef: "U_LONE", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "c"}})
	q := `linked(?a) :- component.net(?a,?n), component.net(?b,?n), ?a != ?b;
	      isolated(?r) :- component.net(?r,?n), not linked(?r);
	      isolated(?r) => ?r`
	rows := runQuery(t, check.NewModel(d), q)
	if len(rows) != 1 || rows[0].Bind["r"].S != "U_LONE" {
		t.Errorf("isolated = %+v, want only U_LONE (every other part shares a net)", rows)
	}
}

// TestUnstratifiable (WS3-029 fast-follow) checks that recursion through negation is rejected. `p`
// needs `not q` and `q` needs `not p` in the same cycle. That cycle has no stratification, so the
// evaluator must say so rather than loop or give an order-dependent answer.
func TestUnstratifiable(t *testing.T) {
	q := `p(?r) :- component.net(?r,?n), not q(?r);
	      q(?r) :- component.net(?r,?n), not p(?r);
	      p(?r) => ?r`
	if _, err := (Naive{}).Eval(context.Background(), mustParse(t, q), NewBase(check.NewModel(chainDesign()))); err == nil {
		t.Error("recursion through negation was accepted; want an unstratifiable error")
	}
}

// TestRuleErrors (WS3-029 fast-follow) checks that a malformed rule program is rejected with a
// clear error, not a silent empty answer.
func TestRuleErrors(t *testing.T) {
	m := check.NewModel(chainDesign())
	cases := map[string]string{
		"redefine EDB":        `component.net(?a,?b) :- component.net(?a,?b); component.net(?a,?b) => ?a`,
		"redefine builtin":    `net.reaches(?a,?b) :- component.net(?a,?b); net.reaches(?a,?b) => ?a`,
		"unsafe head var":     `bad(?x,?y) :- component.net(?x,?n); bad(?x,?y) => ?x`,
		"unknown body rel":    `bad(?x) :- nope(?x); bad(?x) => ?x`,
		"inconsistent arity":  `r(?a) :- component.net(?a,?n); r(?a,?b) :- component.net(?a,?b); r(?a) => ?a`,
		"no goal (all rules)": `r(?a) :- component.net(?a,?n)`,
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			q, err := Parse(text)
			if err != nil {
				return // a parse-level rejection is also acceptable
			}
			if _, err := (Naive{}).Eval(context.Background(), q, NewBase(m)); err == nil {
				t.Errorf("Eval(%q) succeeded; want an error", text)
			}
		})
	}
}

// TestRulesDoNotLeakAcrossQueries (WS3-029 fast-follow) checks that rules materialize onto a
// per-query copy. A Base reused for a second query that defines no rules sees none of the first
// query's IDB.
func TestRulesDoNotLeakAcrossQueries(t *testing.T) {
	b := NewBase(check.NewModel(chainDesign()))
	if _, err := (Naive{}).Eval(context.Background(), mustParse(t, `v(?a) :- component.net(?a,?n); v(?a) => ?a`), b); err != nil {
		t.Fatalf("first query: %v", err)
	}
	// The second query references the same IDB name; it must now be unknown (the rule did not persist).
	if _, err := (Naive{}).Eval(context.Background(), mustParse(t, `v(?a) => ?a`), b); err == nil {
		t.Error("IDB relation v survived into a ruleless query; rules must not leak across queries")
	}
}

// oddLen is an overlay's filter predicate, odd_len(?s), true when the string's length is odd.
var oddLen = ns.Filter(1, func(args []Value) (bool, error) { return len(args[0].S)%2 == 1, nil })

// TestRegisteredPredicateFiltersAndNegates (predicate-interface) registers an overlay's filter
// predicate at a path in the fact layer's vocabulary. The predicate is then a first-class query
// citizen. It filters in the goal, and `not` ranges over it. Both derive from the one boolean, so
// they can never disagree.
//
// The test composes its own registry, so it never writes to the process default and has nothing to
// restore.
func TestRegisteredPredicateFiltersAndNegates(t *testing.T) {
	reg := facts.RegistryWith(facts.WithPredicate("odd_len", oddLen))
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "d"}}, {RefDes: "R22", Prov: &ir.Provenance{SourceFile: "d"}}},
		Nets:       []*ir.Net{{Name: "N", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}, {ComponentRef: "R22", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "d"}}},
	}
	m := check.NewModel(d)
	pos := runQueryOn(t, reg, m, `component.net(?r,?n), odd_len(?r) => ?r`) // R22 (len 3), not U1 (len 2)
	if len(pos) != 1 || pos[0].Bind["r"].S != "R22" {
		t.Errorf("odd_len filter = %+v, want only R22", pos)
	}
	neg := runQueryOn(t, reg, m, `component.net(?r,?n), not odd_len(?r) => ?r`) // U1
	if len(neg) != 1 || neg[0].Bind["r"].S != "U1" {
		t.Errorf("not odd_len = %+v, want only U1", neg)
	}
}

// TestPredicateRegistrationRejects (predicate-interface) checks that a misregistration fails
// composition. facts.RegisterPredicate turns that failure into a panic at load.
func TestPredicateRegistrationRejects(t *testing.T) {
	ok := func([]Value) (bool, error) { return true, nil }
	cases := map[string]struct {
		path string
		b    ns.Builtin
	}{
		"empty name":        {"", ns.Filter(1, ok)},
		"zero arity":        {"p", ns.Filter(0, ok)},
		"nil predicate":     {"p", ns.Builtin{Arity: 1}},
		"collide standard":  {"str.contains", ns.Filter(2, ok)},
		"collide relation":  {"component.net", ns.Filter(2, ok)},
		"collide reaches":   {"net.reaches", ns.Filter(2, ok)},
		"module and member": {"net", ns.Filter(1, ok)},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := facts.NewRegistry(append(facts.Registered(), facts.WithPredicate(c.path, c.b))...); err == nil {
				t.Errorf("composing %s produced no error; want a load-time rejection", name)
			}
		})
	}
}

// TestNegatedReaches (predicate-interface) checks that negation now ranges over reaches too. It is
// negation as failure over extendAtom, which the positive solve also uses. A negated reaches used
// to error.
func TestNegatedReaches(t *testing.T) {
	m := check.NewModel(reachDesign()) // A reaches B through the series resistor
	// A does reach B, so `not net.reaches("A","B")` drops every row.
	if rows := runQuery(t, m, `net.reaches("A",?x), not net.reaches("A","B") => ?x`); len(rows) != 0 {
		t.Errorf("not net.reaches(A,B) kept %+v, want none (A does reach B)", rows)
	}
	// A does not reach a nonexistent net, so `not net.reaches("A","ZZZ")` keeps the rows.
	if rows := runQuery(t, m, `net.reaches("A",?x), not net.reaches("A","ZZZ") => ?x`); len(rows) == 0 {
		t.Error("not net.reaches(A,ZZZ) dropped everything, want the reachable nets kept")
	}
}

// TestRegisterRelation (WS3-029 fast-follow) covers the extension hook. An out-of-engine relation
// registered with RegisterRelation is a first-class query citizen. The goal joins it against a
// built-in relation, a rule reads it, and negation ranges over it, all with no evaluator change.
func TestRegisterRelation(t *testing.T) {
	// An overlay "house.approved(ref)" relation, where U1 is approved and U2 is not. It is composed
	// onto the process vocabulary rather than added to it, so the test owns what it queries.
	reg := facts.RegistryWith(facts.WithRelation("house.approved", []facts.Field{facts.FieldSubject}, func(m check.Model) []facts.Row {
		var out []facts.Row
		for _, c := range m.Components() {
			if c.RefDes == "U1" {
				out = append(out, facts.Row{Subject: c.RefDes, Cites: []string{"house db row 7"}})
			}
		}
		return out
	}))
	m := check.NewModel(chainDesign()) // U1, U2, U3 on shared nets

	// Join the overlay relation against the built-in component.net, and carry its provenance.
	rows := runQueryOn(t, reg, m, `house.approved(?r), component.net(?r,?n) => ?r, ?n`)
	if len(rows) == 0 {
		t.Fatal("overlay relation join produced no rows")
	}
	for _, r := range rows {
		if r.Bind["r"].S != "U1" {
			t.Errorf("row %+v: only U1 is house-approved", r.Bind)
		}
	}
	if !containsCite(rows[0].Cites, "house db row 7") {
		t.Errorf("cites %v missing the overlay provenance", rows[0].Cites)
	}

	// A rule reads the overlay relation, and negation ranges over it: unapproved parts.
	un := runQueryOn(t, reg, m, `unapproved(?r) :- component.net(?r,?n), not house.approved(?r); unapproved(?r) => ?r`)
	got := map[string]bool{}
	for _, r := range un {
		got[r.Bind["r"].S] = true
	}
	if got["U1"] || !got["U2"] || !got["U3"] {
		t.Errorf("unapproved = %v, want U2 and U3 but not U1", got)
	}
}

// TestRegisterRelationRejects (WS3-029 fast-follow) checks that a misregistration fails loudly when
// the vocabulary composes, not silently at query time. The two collision cases need the REAL
// built-in catalog and the REAL reserved predicate names. Those exist only once this package and
// stdlib/relations are both linked in, so the cases are here rather than in core/facts.
func TestRegisterRelationRejects(t *testing.T) {
	nilProj := func(check.Model) []facts.Row { return nil }
	cases := map[string]facts.Option{
		"empty name":       facts.WithRelation("", []facts.Field{facts.FieldSubject}, nilProj),
		"no fields":        facts.WithRelation("x.y", nil, nilProj),
		"nil projector":    facts.WithRelation("x.y", []facts.Field{facts.FieldSubject}, nil),
		"collide built-in": facts.WithRelation("component.mpn", []facts.Field{facts.FieldSubject}, nilProj),
		"collide reaches":  facts.WithRelation("net.reaches", []facts.Field{facts.FieldSubject, facts.FieldObject}, nilProj),
	}
	for name, opt := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := facts.NewRegistry(append(facts.Registered(), opt)...); err == nil {
				t.Errorf("composing %s produced no error; want a load-time rejection", name)
			}
		})
	}
}

func containsCite(cites []string, want string) bool {
	for _, c := range cites {
		if c == want {
			return true
		}
	}
	return false
}

// TestEvalErrors (WS3-029) checks that the evaluator rejects bad queries clearly rather than
// guessing.
func TestEvalErrors(t *testing.T) {
	m := check.NewModel(regDesign("+24V"))
	cases := map[string]string{
		"unknown relation":   `not_a_relation(?x) => ?x`,
		"arity mismatch":     `component.mpn(?a,?b,?c) => ?a`,
		"unbound compare":    `?x < ?y => ?x`,
		"select existential": `component.mpn(?r,?m), not param.max(?m,"VIN",?v) => ?v`,
		"unknown aggregate":  `component.mpn(?r,?m) => ?r, median(?m)`,
		"negate unknown rel": `component.mpn(?r,?m), not nope(?m) => ?m`,
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			q, err := Parse(text)
			if err != nil {
				return // a parse-level rejection is also acceptable
			}
			if _, err := (Naive{}).Eval(context.Background(), q, NewBase(m)); err == nil {
				t.Errorf("Eval(%q) succeeded; want an error", text)
			}
		})
	}
}

// TestComponentClassRelation (WS3-071) checks that component.class returns one row per class tag.
// So a TVS answers both its specific tag and its diode family tag, and a datalog rule joins on the
// family tag to ask membership. The design has a stamped device_classes set, as a loader-read
// design would.
func TestComponentClassRelation(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "D1", DeviceClasses: classify.Tags("tvs", "diode")},
		{RefDes: "R1", DeviceClasses: classify.Tags("resistor")},
	}}
	m := check.NewModel(d)

	all := runQuery(t, m, `component.class(?ref, ?cls) => ?ref, ?cls`)
	if len(all) != 3 {
		t.Fatalf("component.class rows = %d, want 3 (D1 tvs, D1 diode, R1 resistor)", len(all))
	}

	diodeFamily := runQuery(t, m, `component.class(?ref, "diode") => ?ref`)
	if len(diodeFamily) != 1 || diodeFamily[0].Bind[Var("ref")].S != "D1" {
		t.Errorf("component.class(?ref, \"diode\") = %v, want just D1", diodeFamily)
	}
}

// TestBusRelation (WS1-034) checks that the bus(label, kind) relation exposes reader-detected
// unmodeled buses for ad-hoc search. A projection lists them, and a bound kind filters them. An
// anonymous bus wire has an empty label.
func TestBusRelation(t *testing.T) {
	d := &ir.Design{InputDiagnostics: &ir.InputDiagnostics{UnmodeledBuses: []*ir.BusNotModeled{
		{Kind: "bus_alias", Label: "DATA"},
		{Kind: "bus", Label: ""},
	}}}
	m := check.NewModel(d)

	all := runQuery(t, m, `bus(?label, ?kind) => ?label, ?kind`)
	if len(all) != 2 {
		t.Fatalf("bus rows = %d, want 2", len(all))
	}
	aliases := runQuery(t, m, `bus(?label, "bus_alias") => ?label`)
	if len(aliases) != 1 || aliases[0].Bind[Var("label")].S != "DATA" {
		t.Errorf("bus(?label, \"bus_alias\") = %v, want just DATA", aliases)
	}
}

// TestUnanchoredNegationErrors covers the first half of agni issue 522. Imagine a negated atom that
// shares no variable with the rest of the query. It has nothing to range over per row, so it
// collapses to a design-wide constant. It then filters every row away, or none. Before this it
// returned "no results", which reads as a reassuring fact about the design rather than as a
// malformed question.
//
// The shape here is the natural spelling of "nets with no test point". So the author gets a
// confident empty answer to the most ordinary question a review asks.
func TestUnanchoredNegationErrors(t *testing.T) {
	d, set := twoPartDesign()
	m := check.NewModel(d, check.WithParamProvider(set))
	_, err := (Naive{}).Eval(context.Background(), mustParse(t, `component.mpn(?r,?m), not param.max(?x,"VIN",?v) => ?m`), NewBase(m))
	if err == nil {
		t.Fatal("an unanchored negation was accepted; it silently answers nothing")
	}
	// The message has to name the offending variable, because in a long body the author cannot
	// otherwise tell WHICH `not` is the unanchored one.
	if !strings.Contains(err.Error(), "?x") {
		t.Errorf("error = %v, want it to name the unanchored variable ?x", err)
	}
}

// TestAnchoredNegationWithFreeValueStillWorks is the guard on the guard. Classic datalog safety
// says every variable in a negated literal must occur positively. Applying that rule here would
// break a shape that this language documents and people rely on. `not param.max(?m,"VIN",?v)`
// leaves ?v free ON PURPOSE and means "no VIN param for any value". It is well defined because ?m
// anchors it.
//
// So the rule implemented is ANCHORING, not full safety. This test stops someone "fixing" it into
// the stricter form later. TestNegation above covers the same shape's results, and this one exists
// to state the rule.
func TestAnchoredNegationWithFreeValueStillWorks(t *testing.T) {
	d, set := twoPartDesign()
	m := check.NewModel(d, check.WithParamProvider(set))
	if _, err := (Naive{}).Eval(context.Background(), mustParse(t, `component.mpn(?r,?m), not param.max(?m,"VIN",?v) => ?m`), NewBase(m)); err != nil {
		t.Fatalf("an anchored negation with a free value variable must be accepted: %v", err)
	}
}

// TestGroundNegationNeedsNoAnchor covers a negated atom with no variables at all. It is a constant
// filter, and a legitimate one, so the anchoring rule must not reach it.
func TestGroundNegationNeedsNoAnchor(t *testing.T) {
	d, set := twoPartDesign()
	m := check.NewModel(d, check.WithParamProvider(set))
	if _, err := (Naive{}).Eval(context.Background(), mustParse(t, `component.mpn(?r,?m), not param.max("REG-24","VIN",20) => ?m`), NewBase(m)); err != nil {
		t.Fatalf("a ground negation carries no variables to anchor and must be accepted: %v", err)
	}
}

// TestUnanchoredNegationErrorsInARuleBody covers rule bodies. A rule body runs through the same
// solver as a goal, so it gets the same guard. A query authored over the wire arrives as rules.
// Without this guard, it would keep the silent behaviour the goal path just lost.
func TestUnanchoredNegationErrorsInARuleBody(t *testing.T) {
	d, set := twoPartDesign()
	m := check.NewModel(d, check.WithParamProvider(set))
	q := mustParse(t, `bad(?m) => ?m`)
	q.Rules = []Rule{{
		Head: Atom{Relation: "bad", Args: []Term{{Var: "m"}}},
		Body: Body{Literals: []Literal{
			{Pos: &Atom{Relation: "component.mpn", Args: []Term{{Var: "r"}, {Var: "m"}}}},
			{Neg: &Atom{Relation: "param.max", Args: []Term{{Var: "x"}, {Const: &Value{S: "VIN"}}, {Var: "v"}}}},
		}},
	}}
	if _, err := (Naive{}).Eval(context.Background(), q, NewBase(m)); err == nil {
		t.Fatal("an unanchored negation in a rule body was accepted")
	}
}

// aggFixture is a design shaped like the group-filter question. It has three nets whose test-point
// coverage differs. One net has a second capacitor, so a flat goal produces more bindings than
// values.
//
//	TPONLY   TP1                       -> 1 test point,  no capacitor
//	BOTH     TP2, TP3, C1              -> 2 test points, 1 capacitor  (2 bindings, 2 values)
//	DOUBLE   TP4, C2, C3               -> 1 test point,  2 capacitors (2 bindings, 1 value)
func aggFixture() *ir.Design {
	prov := func() *ir.Provenance { return &ir.Provenance{SourceFile: "d"} }
	comp := func(ref, class string) *ir.Component {
		return &ir.Component{RefDes: ref, DeviceClasses: classify.Tags(class), Prov: prov()}
	}
	net := func(name string, refs ...string) *ir.Net {
		n := &ir.Net{Name: name, Prov: prov()}
		for i, r := range refs {
			n.Connections = append(n.Connections, &ir.Connection{ComponentRef: r, PinRef: fmt.Sprint(i + 1)})
		}
		return n
	}
	return &ir.Design{
		Components: []*ir.Component{
			comp("TP1", "test_point"), comp("TP2", "test_point"), comp("TP3", "test_point"), comp("TP4", "test_point"),
			comp("C1", "capacitor"), comp("C2", "capacitor"), comp("C3", "capacitor"),
		},
		Nets: []*ir.Net{
			net("TPONLY", "TP1"),
			net("BOTH", "TP2", "TP3", "C1"),
			net("DOUBLE", "TP4", "C2", "C3"),
		},
	}
}

func aggRows(t *testing.T, q string) map[string]string {
	t.Helper()
	rows := runQuery(t, check.NewModel(aggFixture()), q)
	got := map[string]string{}
	for _, r := range rows {
		key := r.Bind["net"].S
		var vals []string
		for k, v := range r.Bind {
			if k != "net" {
				vals = append(vals, string(k)+"="+v.S)
			}
		}
		sort.Strings(vals)
		got[key] = strings.Join(vals, " ")
	}
	return got
}

// TestHavingFiltersGroups checks that a having filters after the reduce. A goal comparison cannot
// do that, because before grouping there is no count to compare.
func TestHavingFiltersGroups(t *testing.T) {
	got := aggRows(t, `component.class(?tp,"test_point"), component.net(?tp,?net) => ?net, count(?tp) having count(?tp) > 1`)
	if len(got) != 1 || got["BOTH"] != "count(tp)=2" {
		t.Errorf("rows = %v, want only BOTH with count 2", got)
	}
}

// TestHavingWithoutSelectingTheAggregate checks that an aggregate may be filtered on without being
// projected. The answer is then the subjects rather than the tally. The aggregate's column must not
// reach the output.
func TestHavingWithoutSelectingTheAggregate(t *testing.T) {
	q := `component.class(?tp,"test_point"), component.net(?tp,?net) => ?net having count(?tp) = 1`
	rows := runQuery(t, check.NewModel(aggFixture()), q)
	names := []string{}
	for _, r := range rows {
		names = append(names, r.Bind["net"].S)
		if _, leaked := r.Bind["count(tp)"]; leaked {
			t.Errorf("row %v carries the having column, which was not selected", r.Bind)
		}
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "DOUBLE,TPONLY" {
		t.Errorf("nets = %v, want [DOUBLE TPONLY]", names)
	}
	parsed, err := Parse(q)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cols := parsed.Columns(); len(cols) != 1 || cols[0] != "net" {
		t.Errorf("Columns() = %v, want [net]", cols)
	}
}

// TestHavingComparedAgainstAGroupKey checks that the right side may be a selected variable, not
// only a constant. "Nets whose every pin is a test point" asks that. Only TPONLY qualifies. BOTH
// and DOUBLE each have three pins and fewer test points.
func TestHavingComparedAgainstAGroupKey(t *testing.T) {
	rows := runQuery(t, check.NewModel(aggFixture()),
		`component.class(?tp,"test_point"), component.net(?tp,?net), net.pin_count(?net,?pc) => ?net, ?pc having count(?tp) = ?pc`)
	if len(rows) != 1 || rows[0].Bind["net"].S != "TPONLY" {
		t.Errorf("rows = %+v, want only TPONLY", rows)
	}
}

// TestHavingRejectsANonGroupKeyOnTheRight checks that validation catches a non-group key on the
// right. So the query fails on the query itself, rather than on whichever design happens to produce
// the first group.
func TestHavingRejectsANonGroupKeyOnTheRight(t *testing.T) {
	q, perr := Parse(`component.class(?tp,"test_point"), component.net(?tp,?net) => ?net having count(?tp) = ?tp`)
	if perr != nil {
		t.Fatalf("Parse: %v", perr)
	}
	_, err := (Naive{}).Eval(context.Background(), q, NewBase(check.NewModel(aggFixture())))
	if err == nil || !strings.Contains(err.Error(), "group key") {
		t.Errorf("err = %v, want a complaint that ?tp is not a group key", err)
	}
}

// TestDistinctReducesValuesNotBindings covers a known trap, and distinct exists so a query can
// avoid it. DOUBLE has one test point and two capacitors, so the flat goal yields two bindings for
// one value.
func TestDistinctReducesValuesNotBindings(t *testing.T) {
	got := aggRows(t, `component.class(?tp,"test_point"), component.net(?tp,?net),
		component.class(?c,"capacitor"), component.net(?c,?net) => ?net, count(?tp), count(distinct ?tp)`)
	if got["DOUBLE"] != "count(distinct tp)=1 count(tp)=2" {
		t.Errorf("DOUBLE = %q, want 2 bindings reduced to 1 distinct value", got["DOUBLE"])
	}
	if got["BOTH"] != "count(distinct tp)=2 count(tp)=2" {
		t.Errorf("BOTH = %q, want both spellings to agree when every binding is a fresh value", got["BOTH"])
	}
}

// TestListNamesTheGroupMembers checks that list answers a list value of the members, sorted, in its
// JSON text. list is binding-wise until asked otherwise, the same rule count follows. So the two
// columns describe the same set.
func TestListNamesTheGroupMembers(t *testing.T) {
	got := aggRows(t, `component.class(?tp,"test_point"), component.net(?tp,?net),
		component.class(?c,"capacitor"), component.net(?c,?net) => ?net, list(?tp), list(distinct ?tp)`)
	if got["DOUBLE"] != `list(distinct tp)=["TP4"] list(tp)=["TP4","TP4"]` {
		t.Errorf("DOUBLE = %q, want the bare list to repeat per binding and distinct to collapse it", got["DOUBLE"])
	}
	if got["BOTH"] != `list(distinct tp)=["TP2","TP3"] list(tp)=["TP2","TP3"]` {
		t.Errorf("BOTH = %q, want sorted members", got["BOTH"])
	}
}

// TestSumDistinctReducesDistinctValues checks that distinct is uniform across the aggregates, not a
// count special case. Two bindings with the same number contribute once.
func TestSumDistinctReducesDistinctValues(t *testing.T) {
	got := aggRows(t, `component.class(?tp,"test_point"), component.net(?tp,?net),
		net.pin_count(?net,?pc) => ?net, sum(?pc), sum(distinct ?pc)`)
	if got["BOTH"] != "sum(distinct pc)=3 sum(pc)=6" {
		t.Errorf("BOTH = %q, want the repeated pin count summed once when distinct", got["BOTH"])
	}
}
