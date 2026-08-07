package query

import (
	"fmt"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/datasheet/param"
)

// boardGeom is a two-net board: SIG routed thin (0.08mm), PWR wide (0.5mm).
func boardGeom() *geom.BoardGeometry {
	return &geom.BoardGeometry{UnitNm: 1, Nets: []*geom.NetCopper{
		{Net: "SIG", Segments: []*geom.TrackSegment{{Width: 80000, Layer: "F.Cu"}}},
		{Net: "PWR", Segments: []*geom.TrackSegment{{Width: 500000, Layer: "F.Cu"}}},
	}}
}

// TestBoardQuery (WS1-041): a board relation is queryable through the same engine with no evaluator
// change — nets routed thinner than 0.1mm.
func TestBoardQuery(t *testing.T) {
	rows := runQuery(t, check.NewModelWithBoard(&ir.Design{}, boardGeom()), `board.track_width(?net,?w), ?w < 0.1 => ?net, ?w`)
	if len(rows) != 1 || rows[0].Bind["net"].S != "SIG" {
		t.Errorf("rows = %+v, want SIG (0.08mm thin)", rows)
	}
}

// TestCrossTierJoin (WS1-041): one query joins BOARD ⋈ NETLIST ⋈ DATASHEET — a thin-routed net, the
// part on it, and its datasheet identity — and the answer cites the board and the schematic. This
// is the payoff of tier-generality: no vendor offers a query that spans copper, connectivity, and
// datasheets at once.
func TestCrossTierJoin(t *testing.T) {
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "U1", Attributes: map[string]string{"MPN": "REG-24"}, Prov: &ir.Provenance{SourceFile: "x"}}},
		Nets:       []*ir.Net{{Name: "SIG", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "x"}}},
	}
	m := check.NewModelWithParams(d, boardGeom(), param.ParamSet{"REG-24": regSpec("REG-24", 20)})
	rows := runQuery(t, m, `board.track_width(?net,?w), component-on-net(?ref,?net), component.mpn(?ref,?mpn), ?w < 0.1 => ?net, ?ref, ?mpn`)
	if len(rows) != 1 || rows[0].Bind["ref"].S != "U1" || rows[0].Bind["mpn"].S != "REG-24" || rows[0].Bind["net"].S != "SIG" {
		t.Fatalf("cross-tier rows = %+v, want SIG/U1/REG-24", rows)
	}
	joined := strings.Join(rows[0].Cites, " | ")
	if !strings.Contains(joined, "board net SIG") || !strings.Contains(joined, "x") {
		t.Errorf("cites %q should span board (board net SIG) and netlist (x)", joined)
	}
}

func f64(x float64) *float64 { return &x }

// regSpec is a seeded part with one absolute-maximum VIN row — the datasheet side of the showcase
// join, cited to a page.
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
		Components: []*ir.Component{{RefDes: "U1", Attributes: map[string]string{"MPN": "REG-24"}, Prov: &ir.Provenance{SourceFile: "reg"}}},
		Nets:       []*ir.Net{{Name: railNet, Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "reg"}}},
	}
}

func runQuery(t *testing.T, m check.Model, text string) []Row {
	t.Helper()
	q, err := Parse(text)
	if err != nil {
		t.Fatalf("Parse(%q): %v", text, err)
	}
	rows, err := (Naive{}).Eval(q, NewBase(m))
	if err != nil {
		t.Fatalf("Eval(%q): %v", text, err)
	}
	return rows
}

// TestShowcaseJoin (WS3-029): the flagship datasheet query — a part whose abs-max VIN is below the
// rail it sits on — joins param ⋈ component.mpn ⋈ component-on-net ⋈ net.max_voltage and the answer
// carries provenance. This is "search your design incl. datasheets, with verifiability" made real.
func TestShowcaseJoin(t *testing.T) {
	m := check.NewModelWithParams(regDesign("+24V"), nil, param.ParamSet{"REG-24": regSpec("REG-24", 20)})
	rows := runQuery(t, m,
		`component.mpn(?ref,?mpn), param(?mpn,"VIN",?vmax), component-on-net(?ref,?net), net.max_voltage(?net,?rail), ?vmax < ?rail => ?ref, ?vmax, ?net, ?rail`)

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

// TestShowcasePasses (WS3-029): the same query is silent when the rail is within the abs-max — the
// comparison prunes, so no answer is an answer.
func TestShowcasePasses(t *testing.T) {
	m := check.NewModelWithParams(regDesign("+12V"), nil, param.ParamSet{"REG-24": regSpec("REG-24", 20)})
	rows := runQuery(t, m,
		`component.mpn(?ref,?mpn), param(?mpn,"VIN",?vmax), component-on-net(?ref,?net), net.max_voltage(?net,?rail), ?vmax < ?rail => ?ref`)
	if len(rows) != 0 {
		t.Errorf("rows = %v, want none (20V abs-max, 12V rail)", rows)
	}
}

// vddSpec is REG-24 with TWO rows on the ONE symbol VDD: a recommended-operating window (3.0..3.6)
// and an absolute-maximum ceiling (4.6). This is exactly the case the thin param(mpn,symbol,max)
// relation cannot tell apart — both surface as param(REG-24,"VDD",...) — and that param.range
// separates by its kind argument.
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

// TestParamRangeTwoSidedJoin (WS3-082) demonstrates that a two-sided, limit-kind-discriminated
// range check IS authorable in datalog with param.range + net.nominal_voltage — the join the thin
// param(mpn,symbol,max) relation could not express (no lower bound, no way to tell an absolute-max
// row from a recommended-operating one on the same symbol). This is the ticket's "demonstrated
// datalog program" proof that the enriched vocabulary is sufficient for the datasheet-range rule
// family.
func TestParamRangeTwoSidedJoin(t *testing.T) {
	m := check.NewModelWithParams(regDesign("+5V"), nil, param.ParamSet{"REG-24": vddSpec("REG-24")})

	// Over the recommended maximum: +5V > 3.6. The "recommended_operating" kind filter keeps the
	// abs-max row (4.6) out of this check, so exactly one answer.
	over := runQuery(t, m,
		`component.mpn(?ref,?mpn), param.range(?mpn,?sym,"recommended_operating",?min,?max), component-on-net(?ref,?net), net.nominal_voltage(?net,?v), ?v > ?max => ?ref, ?net, ?v, ?max`)
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

	// Under the recommended MINIMUM — the side the thin param relation carries no bound for. A +2V5
	// rail on the same part is below the 3.0 floor.
	under := runQuery(t, check.NewModelWithParams(regDesign("+2V5"), nil, param.ParamSet{"REG-24": vddSpec("REG-24")}),
		`component.mpn(?ref,?mpn), param.range(?mpn,?sym,"recommended_operating",?min,?max), component-on-net(?ref,?net), net.nominal_voltage(?net,?v), ?v < ?min => ?ref, ?v, ?min`)
	if len(under) != 1 || under[0].Bind["min"].Num == nil || *under[0].Bind["min"].Num != 3.0 {
		t.Fatalf("recommended under-min: rows = %v, want one with min 3.0", under)
	}

	// Kind discrimination: filtering the SAME symbol by kind picks the abs-max ceiling (4.6), which
	// param(mpn,"VDD",max) could never separate from the recommended 3.6.
	abs := runQuery(t, m, `param.range("REG-24","VDD","absolute_max",?min,?max) => ?max`)
	if len(abs) != 1 || abs[0].Bind["max"].Num == nil || *abs[0].Bind["max"].Num != 4.6 {
		t.Fatalf("abs-max row: rows = %v, want one with max 4.6", abs)
	}
}

// TestSingleRelation (WS3-029): a one-atom query is a plain select-project over the fact base.
func TestSingleRelation(t *testing.T) {
	m := check.NewModelWithParams(regDesign("+24V"), nil, param.ParamSet{"REG-24": regSpec("REG-24", 20)})
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

// TestReachesRecursion (WS3-029): the built-in reaches relation (bridged to check.Model.Reach)
// makes recursion real — reaches from A crosses the series resistor to B (reflexive, so A too).
func TestReachesRecursion(t *testing.T) {
	rows := runQuery(t, check.NewModel(reachDesign()), `reaches("A",?n) => ?n`)
	got := map[string]bool{}
	for _, r := range rows {
		got[r.Bind["n"].S] = true
	}
	if !got["A"] || !got["B"] {
		t.Errorf("reaches(A) = %v, want A and B (through the series resistor)", got)
	}
}

// reachChainDesign is a straight series chain N0 -R1- N1 -R2- N2 -R3- N3, so every net sits at a
// known, distinct distance from N0. The graded distance is the point: a fixture where everything is
// one hop away cannot tell a radius filter from a no-op.
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

// TestReachesBindsDistance (WS3-112): the optional third argument binds the number of series
// crossings, reflexive at 0, so a rule states its own radius instead of inheriting the engine's.
func TestReachesBindsDistance(t *testing.T) {
	rows := runQuery(t, check.NewModel(reachChainDesign()), `reaches("N0", ?n, ?h) => ?n, ?h`)
	got := map[string]string{}
	for _, r := range rows {
		got[r.Bind["n"].S] = r.Bind["h"].S
	}
	want := map[string]string{"N0": "0", "N1": "1", "N2": "2", "N3": "3"}
	for n, w := range want {
		if got[n] != w {
			t.Errorf("reaches(N0, %s, ?h) bound %q, want %q (full: %v)", n, got[n], w, got)
		}
	}
}

// TestReachesRadiusFilter (WS3-112) is the test that would have caught composing a 2-hop protection
// predicate out of the 100-hop reaches built-in: a clamp three series elements away must NOT satisfy
// a "within two hops" question, and reporting that it does is a false PASS on a real defect.
func TestReachesRadiusFilter(t *testing.T) {
	rows := runQuery(t, check.NewModel(reachChainDesign()), `reaches("N0", ?n, ?h), ?h <= 2 => ?n`)
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

// TestReachesConstantHopsIsExact (WS3-112) pins the semantics the doc and catalog warn about: a bare
// constant in the third slot binds by equality, so it means EXACTLY that many hops. A reader who
// writes it expecting "within" gets a silently narrower answer, which is why the radius idiom is a
// comparison.
func TestReachesConstantHopsIsExact(t *testing.T) {
	rows := runQuery(t, check.NewModel(reachChainDesign()), `reaches("N0", ?n, 2) => ?n`)
	got := map[string]bool{}
	for _, r := range rows {
		got[r.Bind["n"].S] = true
	}
	if len(got) != 1 || !got["N2"] {
		t.Errorf("reaches(N0, ?n, 2) = %v, want exactly {N2} (equality, not a bound)", got)
	}
}

// TestReachesArityBothPathsAgree (WS3-112): the optional argument is admitted by the POSITIVE path
// and the NEGATION path through one predicate. A divergence here would accept reaches(?a,?b,?h) while
// rejecting `not reaches(?a,?b,?h)`, and negation is validated up front so it would fail the whole
// query rather than degrade.
func TestReachesArityBothPathsAgree(t *testing.T) {
	m := check.NewModel(reachChainDesign())
	rows := runQuery(t, m, `component-on-net(?r, ?n), not reaches("N0", ?n, ?h) => ?n`)
	for _, r := range rows {
		if n := r.Bind["n"].S; n == "N1" {
			t.Errorf("not reaches(N0, N1, ?h) should not hold: %v", rows)
		}
	}
	q, err := Parse(`reaches(?a, ?b, ?c, ?d) => ?a`)
	if err == nil {
		if _, err = (Naive{}).Eval(q, NewBase(m)); err == nil {
			t.Error("reaches at arity 4: want an error naming the accepted arity")
		}
	}
}

// twoPartDesign places U1 (has a VIN spec) and U2 (no spec) on separate rails.
func twoPartDesign() (*ir.Design, param.ParamSet) {
	d := &ir.Design{
		Components: []*ir.Component{
			{RefDes: "U1", Attributes: map[string]string{"MPN": "REG-24"}, Prov: &ir.Provenance{SourceFile: "d"}},
			{RefDes: "U2", Attributes: map[string]string{"MPN": "PLAIN"}, Prov: &ir.Provenance{SourceFile: "d"}},
		},
		Nets: []*ir.Net{
			{Name: "+24V", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "d"}},
			{Name: "+5V", Connections: []*ir.Connection{{ComponentRef: "U2", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "d"}},
		},
	}
	// Only REG-24 is seeded (PLAIN has no spec -> no param facts), so U2's MPN has no VIN param.
	return d, param.ParamSet{"REG-24": regSpec("REG-24", 20)}
}

// TestNegation (WS3-029 fast-follow): `not param(?m,"VIN",?v)` keeps the mpns with NO VIN param.
// The negated ?v appears only under negation, so it is an existential wildcard ("no VIN param for
// any value"); ?m is bound by the positive literal and must match.
func TestNegation(t *testing.T) {
	d, set := twoPartDesign()
	m := check.NewModelWithParams(d, nil, set)
	rows := runQuery(t, m, `component.mpn(?r,?m), not param(?m,"VIN",?v) => ?m`)
	if len(rows) != 1 || rows[0].Bind["m"].S != "PLAIN" {
		t.Errorf("rows = %+v, want only PLAIN (REG-24 has a VIN param, so it is excluded)", rows)
	}
	if len(rows) == 1 && len(rows[0].Cites) == 0 {
		t.Error("a negation answer still carries the positive facts' provenance")
	}
}

// TestAggregationCount (WS3-029 fast-follow): count(?ref) grouped by ?net gives parts-per-net.
func TestAggregationCount(t *testing.T) {
	// Both U1 and U2 sit on SHARED, plus their own rails.
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "d"}}, {RefDes: "U2", Prov: &ir.Provenance{SourceFile: "d"}}},
		Nets: []*ir.Net{
			{Name: "SHARED", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}, {ComponentRef: "U2", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "d"}},
			{Name: "LONE", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "2"}}, Prov: &ir.Provenance{SourceFile: "d"}},
		},
	}
	rows := runQuery(t, check.NewModel(d), `component-on-net(?r,?n) => ?n, count(?r)`)
	got := map[string]string{}
	for _, r := range rows {
		got[r.Bind["n"].S] = r.Bind["count(r)"].S
	}
	if got["SHARED"] != "2" || got["LONE"] != "1" {
		t.Errorf("counts = %v, want SHARED=2 LONE=1", got)
	}
}

// TestAggregationMax (WS3-029 fast-follow): max over a numeric column reduces per group.
func TestAggregationMax(t *testing.T) {
	spec := regSpec("REG-24", 20)
	// add a second numeric param so max has something to choose
	spec.Parameters = append(spec.Parameters, &parampb.Parameter{
		Name: "Iout", Symbol: "IOUT", LimitKind: parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX,
		Value: &parampb.RangeValue{Max: f64(800)}, Unit: "mA",
		Prov: &parampb.ParamProvenance{DocRef: "ds", Page: 4, Method: "hand", Confidence: 1},
	})
	m := check.NewModelWithParams(regDesign("+24V"), nil, param.ParamSet{"REG-24": spec})
	rows := runQuery(t, m, `param(?mpn,?sym,?max) => ?mpn, max(?max)`)
	if len(rows) != 1 || rows[0].Bind["max(max)"].S != "800" {
		t.Errorf("rows = %+v, want one with max=800", rows)
	}
}

// TestStringPredicates (WS3-029 fast-follow): contains/prefix/suffix filter a bound string. Both
// positive and negated forms work, so a search box can ask "MPN contains LM" or "not".
func TestStringPredicates(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "U1", Attributes: map[string]string{"MPN": "LM1117"}, Prov: &ir.Provenance{SourceFile: "d"}},
		{RefDes: "U2", Attributes: map[string]string{"MPN": "REG-24"}, Prov: &ir.Provenance{SourceFile: "d"}},
	}}
	m := check.NewModelWithParams(d, nil, param.ParamSet{"LM1117": regSpec("LM1117", 20), "REG-24": regSpec("REG-24", 20)})

	one := func(text, wantRef string) {
		t.Helper()
		rows := runQuery(t, m, text)
		if len(rows) != 1 || rows[0].Bind["r"].S != wantRef {
			t.Errorf("%s => %+v, want only %s", text, rows, wantRef)
		}
	}
	one(`component.mpn(?r,?m), contains(?m,"LM") => ?r`, "U1")
	one(`component.mpn(?r,?m), prefix(?m,"REG") => ?r`, "U2")
	one(`component.mpn(?r,?m), suffix(?m,"1117") => ?r`, "U1")
	one(`component.mpn(?r,?m), not contains(?m,"LM") => ?r`, "U2") // negated string predicate
}

// TestStringPredicateUnbound (WS3-029 fast-follow): a string predicate whose value is not bound by
// a relation errors clearly (it filters, it cannot enumerate every string).
func TestStringPredicateUnbound(t *testing.T) {
	if _, err := (Naive{}).Eval(mustParse(t, `contains(?x,"LM") => ?x`), NewBase(check.NewModel(&ir.Design{}))); err == nil {
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

// chainDesign wires U1-U2 on net N1 and U2-U3 on net N2: U1 and U3 share no net, so they are linked
// only transitively (through U2). It is the fixture for recursion — a transitive closure reaches U3
// from U1 where a single join cannot.
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

// TestUserRuleView (WS3-029 fast-follow): a non-recursive user rule is a named view — `sharesnet`
// derives the pairs of distinct components on a common net, and the goal queries that IDB relation.
func TestUserRuleView(t *testing.T) {
	rows := runQuery(t, check.NewModel(chainDesign()),
		`sharesnet(?a,?b) :- component-on-net(?a,?n), component-on-net(?b,?n), ?a != ?b; sharesnet("U1",?x) => ?x`)
	got := map[string]bool{}
	for _, r := range rows {
		got[r.Bind["x"].S] = true
	}
	if len(got) != 1 || !got["U2"] {
		t.Errorf("sharesnet(U1) = %v, want only U2 (U1 shares N1 with U2, nothing else directly)", got)
	}
}

// TestUserRuleRecursion (WS3-029 fast-follow): the flagship — a user-defined transitive closure.
// `connected` is its own body atom, so it runs to fixpoint and reaches U3 from U1 through U2, which
// no single join can. The derived answer still carries the base nets' provenance.
func TestUserRuleRecursion(t *testing.T) {
	q := `link(?a,?b) :- component-on-net(?a,?n), component-on-net(?b,?n), ?a != ?b;
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

// TestStratifiedNegationInRule (WS3-029 fast-follow): a rule may negate another IDB relation, and
// stratification derives that relation fully first. `linked` collects components that share a net
// with someone; `isolated` is a component that is NOT linked. U_LONE sits on its own net, so it is
// the only isolated part.
func TestStratifiedNegationInRule(t *testing.T) {
	d := chainDesign()
	d.Components = append(d.Components, &ir.Component{RefDes: "U_LONE", Prov: &ir.Provenance{SourceFile: "c"}})
	d.Nets = append(d.Nets, &ir.Net{Name: "N3", Connections: []*ir.Connection{{ComponentRef: "U_LONE", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "c"}})
	q := `linked(?a) :- component-on-net(?a,?n), component-on-net(?b,?n), ?a != ?b;
	      isolated(?r) :- component-on-net(?r,?n), not linked(?r);
	      isolated(?r) => ?r`
	rows := runQuery(t, check.NewModel(d), q)
	if len(rows) != 1 || rows[0].Bind["r"].S != "U_LONE" {
		t.Errorf("isolated = %+v, want only U_LONE (every other part shares a net)", rows)
	}
}

// TestUnstratifiable (WS3-029 fast-follow): recursion through negation is rejected. `p` needs `not q`
// and `q` needs `not p` in the same cycle, which has no stratification — the evaluator must say so
// rather than loop or give an order-dependent answer.
func TestUnstratifiable(t *testing.T) {
	q := `p(?r) :- component-on-net(?r,?n), not q(?r);
	      q(?r) :- component-on-net(?r,?n), not p(?r);
	      p(?r) => ?r`
	if _, err := (Naive{}).Eval(mustParse(t, q), NewBase(check.NewModel(chainDesign()))); err == nil {
		t.Error("recursion through negation was accepted; want an unstratifiable error")
	}
}

// TestRuleErrors (WS3-029 fast-follow): a malformed rule program is rejected with a clear error, not
// a silent empty answer.
func TestRuleErrors(t *testing.T) {
	m := check.NewModel(chainDesign())
	cases := map[string]string{
		"redefine EDB":        `component-on-net(?a,?b) :- component-on-net(?a,?b); component-on-net(?a,?b) => ?a`,
		"redefine builtin":    `reaches(?a,?b) :- component-on-net(?a,?b); reaches(?a,?b) => ?a`,
		"unsafe head var":     `bad(?x,?y) :- component-on-net(?x,?n); bad(?x,?y) => ?x`,
		"unknown body rel":    `bad(?x) :- nope(?x); bad(?x) => ?x`,
		"inconsistent arity":  `r(?a) :- component-on-net(?a,?n); r(?a,?b) :- component-on-net(?a,?b); r(?a) => ?a`,
		"no goal (all rules)": `r(?a) :- component-on-net(?a,?n)`,
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			q, err := Parse(text)
			if err != nil {
				return // a parse-level rejection is also acceptable
			}
			if _, err := (Naive{}).Eval(q, NewBase(m)); err == nil {
				t.Errorf("Eval(%q) succeeded; want an error", text)
			}
		})
	}
}

// TestRulesDoNotLeakAcrossQueries (WS3-029 fast-follow): rules materialize onto a per-query copy, so
// a Base reused for a second query that defines no rules sees none of the first query's IDB.
func TestRulesDoNotLeakAcrossQueries(t *testing.T) {
	b := NewBase(check.NewModel(chainDesign()))
	if _, err := (Naive{}).Eval(mustParse(t, `v(?a) :- component-on-net(?a,?n); v(?a) => ?a`), b); err != nil {
		t.Fatalf("first query: %v", err)
	}
	// The second query references the same IDB name; it must now be unknown (the rule did not persist).
	if _, err := (Naive{}).Eval(mustParse(t, `v(?a) => ?a`), b); err == nil {
		t.Error("IDB relation v survived into a ruleless query; rules must not leak across queries")
	}
}

// withCleanRegistry snapshots the overlay registry AND the builtins map, restoring both after the
// test so a registered relation or predicate does not leak into other tests. RegisterRelation and
// RegisterPredicate mutate these package maps, so they are cloned (not just aliased) before the test
// registers into them; the builtins clone keeps the standard reaches/contains/prefix/suffix entries.
func withCleanRegistry(t *testing.T) {
	t.Helper()
	origReg, origOrder, origBI := registry, registryOrder, builtins
	reg := make(map[string]relationDef, len(origReg))
	for k, v := range origReg {
		reg[k] = v
	}
	bi := make(map[string]builtin, len(origBI))
	for k, v := range origBI {
		bi[k] = v
	}
	registry, registryOrder, builtins = reg, append([]string(nil), origOrder...), bi
	t.Cleanup(func() { registry, registryOrder, builtins = origReg, origOrder, origBI })
}

// TestRegisterPredicate (predicate-interface): an overlay filter predicate registered with
// RegisterPredicate is a first-class query citizen — it filters in the goal and `not` ranges over it,
// both derived from the one boolean so they can never disagree.
func TestRegisterPredicate(t *testing.T) {
	withCleanRegistry(t)
	// odd_len(?s): true when the string length is odd. A pure filter over the argument value.
	RegisterPredicate("odd_len", 1, func(args []Value) (bool, error) { return len(args[0].S)%2 == 1, nil })
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "d"}}, {RefDes: "R22", Prov: &ir.Provenance{SourceFile: "d"}}},
		Nets:       []*ir.Net{{Name: "N", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}, {ComponentRef: "R22", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "d"}}},
	}
	m := check.NewModel(d)
	pos := runQuery(t, m, `component-on-net(?r,?n), odd_len(?r) => ?r`) // R22 (len 3), not U1 (len 2)
	if len(pos) != 1 || pos[0].Bind["r"].S != "R22" {
		t.Errorf("odd_len filter = %+v, want only R22", pos)
	}
	neg := runQuery(t, m, `component-on-net(?r,?n), not odd_len(?r) => ?r`) // U1
	if len(neg) != 1 || neg[0].Bind["r"].S != "U1" {
		t.Errorf("not odd_len = %+v, want only U1", neg)
	}
}

// TestRegisterPredicateRejects (predicate-interface): a misregistration fails loudly at load.
func TestRegisterPredicateRejects(t *testing.T) {
	cases := map[string]func(){
		"empty name":       func() { RegisterPredicate("", 1, func([]Value) (bool, error) { return true, nil }) },
		"zero arity":       func() { RegisterPredicate("p", 0, func([]Value) (bool, error) { return true, nil }) },
		"nil predicate":    func() { RegisterPredicate("p", 1, nil) },
		"collide built-in": func() { RegisterPredicate("contains", 2, func([]Value) (bool, error) { return true, nil }) },
		"collide EDB": func() {
			RegisterPredicate("component-on-net", 2, func([]Value) (bool, error) { return true, nil })
		},
		"collide reaches": func() { RegisterPredicate("reaches", 2, func([]Value) (bool, error) { return true, nil }) },
	}
	for name, register := range cases {
		t.Run(name, func(t *testing.T) {
			withCleanRegistry(t)
			defer func() {
				if recover() == nil {
					t.Errorf("RegisterPredicate(%s) did not panic; want a load-time rejection", name)
				}
			}()
			register()
		})
	}
}

// TestNegatedReaches (predicate-interface): negation now ranges over reaches too (negation as
// failure over the same extendAtom the positive solve uses) — previously a negated reaches errored.
func TestNegatedReaches(t *testing.T) {
	m := check.NewModel(reachDesign()) // A reaches B through the series resistor
	// A does reach B, so `not reaches("A","B")` drops every row.
	if rows := runQuery(t, m, `reaches("A",?x), not reaches("A","B") => ?x`); len(rows) != 0 {
		t.Errorf("not reaches(A,B) kept %+v, want none (A does reach B)", rows)
	}
	// A does not reach a nonexistent net, so `not reaches("A","ZZZ")` keeps the rows.
	if rows := runQuery(t, m, `reaches("A",?x), not reaches("A","ZZZ") => ?x`); len(rows) == 0 {
		t.Error("not reaches(A,ZZZ) dropped everything, want the reachable nets kept")
	}
}

// TestRegisterRelation (WS3-029 fast-follow): the overlay seam. An out-of-engine relation registered
// with RegisterRelation is a first-class query citizen — the goal joins it against a built-in
// relation, a rule reads it, and negation ranges over it, all with no evaluator change.
func TestRegisterRelation(t *testing.T) {
	withCleanRegistry(t)
	// An overlay "house.approved(ref)" relation: U1 is approved, U2 is not.
	RegisterRelation("house.approved", []Field{FieldSubject}, func(m check.Model) []FactRow {
		var out []FactRow
		for _, c := range m.Components() {
			if c.RefDes == "U1" {
				out = append(out, FactRow{Subject: c.RefDes, Cite: "house db row 7"})
			}
		}
		return out
	})
	m := check.NewModel(chainDesign()) // U1, U2, U3 on shared nets

	// Join the overlay relation against the built-in component-on-net, and carry its provenance.
	rows := runQuery(t, m, `house.approved(?r), component-on-net(?r,?n) => ?r, ?n`)
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
	un := runQuery(t, m, `unapproved(?r) :- component-on-net(?r,?n), not house.approved(?r); unapproved(?r) => ?r`)
	got := map[string]bool{}
	for _, r := range un {
		got[r.Bind["r"].S] = true
	}
	if got["U1"] || !got["U2"] || !got["U3"] {
		t.Errorf("unapproved = %v, want U2 and U3 but not U1", got)
	}
}

// TestRegisterRelationRejects (WS3-029 fast-follow): a misregistration fails loudly at load, not
// silently at query time.
func TestRegisterRelationRejects(t *testing.T) {
	cases := map[string]func(){
		"empty name":    func() { RegisterRelation("", []Field{FieldSubject}, func(check.Model) []FactRow { return nil }) },
		"no fields":     func() { RegisterRelation("x.y", nil, func(check.Model) []FactRow { return nil }) },
		"nil projector": func() { RegisterRelation("x.y", []Field{FieldSubject}, nil) },
		"collide built-in": func() {
			RegisterRelation("component.mpn", []Field{FieldSubject}, func(check.Model) []FactRow { return nil })
		},
		"collide reaches": func() {
			RegisterRelation("reaches", []Field{FieldSubject, FieldObject}, func(check.Model) []FactRow { return nil })
		},
	}
	for name, register := range cases {
		t.Run(name, func(t *testing.T) {
			withCleanRegistry(t)
			defer func() {
				if recover() == nil {
					t.Errorf("RegisterRelation(%s) did not panic; want a load-time rejection", name)
				}
			}()
			register()
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

// TestEvalErrors (WS3-029): the evaluator rejects bad queries clearly rather than guessing.
func TestEvalErrors(t *testing.T) {
	m := check.NewModel(regDesign("+24V"))
	cases := map[string]string{
		"unknown relation":   `not_a_relation(?x) => ?x`,
		"arity mismatch":     `component.mpn(?a,?b,?c) => ?a`,
		"unbound compare":    `?x < ?y => ?x`,
		"select existential": `component.mpn(?r,?m), not param(?m,"VIN",?v) => ?v`,
		"unknown aggregate":  `component.mpn(?r,?m) => ?r, avg(?m)`,
		"negate unknown rel": `component.mpn(?r,?m), not nope(?m) => ?m`,
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			q, err := Parse(text)
			if err != nil {
				return // a parse-level rejection is also acceptable
			}
			if _, err := (Naive{}).Eval(q, NewBase(m)); err == nil {
				t.Errorf("Eval(%q) succeeded; want an error", text)
			}
		})
	}
}

// TestComponentClassRelation (WS3-071): component.class returns one row per class tag, so a TVS
// answers both its specific tag and its diode family tag — a datalog rule joins on the family tag to
// ask membership. The design carries a stamped device_classes set (as a loader-read design would).
func TestComponentClassRelation(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "D1", DeviceClasses: []string{"tvs", "diode"}},
		{RefDes: "R1", DeviceClasses: []string{"resistor"}},
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

// TestBusRelation (WS1-034): the bus(label, kind) relation exposes reader-detected unmodeled buses
// for ad-hoc search — a projection lists them, a bound kind filters. An anonymous bus wire has an
// empty label.
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
