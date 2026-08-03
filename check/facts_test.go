package check

import (
	"reflect"
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
	"github.com/panyam/agni/datasheet/param"
)

// factsByRelation indexes a projection by relation name for the assertions below.
func factsByRelation(fs []FactRow) map[string][]FactRow {
	out := map[string][]FactRow{}
	for _, f := range fs {
		out[f.Relation] = append(out[f.Relation], f)
	}
	return out
}

// TestParamRangeAndNominalFacts (WS3-082): the two enriched relations project as expected — a
// recommended-operating row keeps BOTH bounds and its kind token, and a named rail yields its
// name-derived nominal. These are the facts the datasheet-range family joins.
func TestParamRangeAndNominalFacts(t *testing.T) {
	set := param.ParamSet{"ACME-33": ldoRecommendedSpec("ACME-33", 3.0, 3.6)}
	m := NewModelWithParams(supplyDesign("+5V", false, "ACME-33"), nil, set)
	byRel := factsByRelation(Facts(m))

	// param.range(ACME-33, VDD, recommended_operating, 3.0, 3.6) — kind in Value, min in Min, max in Num.
	pr := byRel[RelParamRange]
	if len(pr) != 1 {
		t.Fatalf("param.range = %+v, want one", pr)
	}
	r := pr[0]
	if r.Subject != "ACME-33" || r.Object != "VDD" || r.Value != "recommended_operating" {
		t.Errorf("param.range subject/symbol/kind = %s/%s/%s, want ACME-33/VDD/recommended_operating", r.Subject, r.Object, r.Value)
	}
	if r.Min == nil || *r.Min != 3.0 || r.Num == nil || *r.Num != 3.6 {
		t.Errorf("param.range min/max = %v/%v, want 3/3.6", r.Min, r.Num)
	}

	// net.nominal_voltage(+5V, 5) — a name-derived nominal, netlist-tier (no --params needed).
	found := false
	for _, f := range byRel[RelNetNominalVoltage] {
		if f.Subject == "+5V" {
			found = true
			if f.Num == nil || *f.Num != 5 {
				t.Errorf("net.nominal_voltage(+5V) num = %v, want 5", f.Num)
			}
		}
	}
	if !found {
		t.Errorf("net.nominal_voltage missing a +5V row: %+v", byRel[RelNetNominalVoltage])
	}
}

// TestFactsProjectsSeedRelations (WS3-004): the four seed relations are derived from a
// datasheet-joined design with the right subjects, numeric values, param conditions, and
// provenance. This is the same design + seeded spec the cap-voltage rule runs on, so the facts
// are exactly the reads that rule declares — now materialized as tuples.
func TestFactsProjectsSeedRelations(t *testing.T) {
	set := param.ParamSet{"DEMO-CAP-6V3": capSpec("DEMO-CAP-6V3", 6.3)}
	m := NewModelWithParams(capDesign("+10V", "DEMO-CAP-6V3"), nil, set)
	byRel := factsByRelation(Facts(m))

	// net.max_voltage(+10V, 10) — GND yields none (no voltage token), so exactly one.
	nv := byRel[RelNetMaxVoltage]
	if len(nv) != 1 || nv[0].Subject != "+10V" || nv[0].Num == nil || *nv[0].Num != 10 {
		t.Errorf("net.max_voltage = %+v, want one (+10V, 10)", nv)
	}

	// component.mpn(C1, DEMO-CAP-6V3)
	mp := byRel[RelComponentMPN]
	if len(mp) != 1 || mp[0].Subject != "C1" || mp[0].Value != "DEMO-CAP-6V3" {
		t.Errorf("component.mpn = %+v, want one (C1, DEMO-CAP-6V3)", mp)
	}

	// param(DEMO-CAP-6V3, VDC, <=6.3, TA=25C) cited to the datasheet page.
	pf := byRel[RelParam]
	if len(pf) != 1 {
		t.Fatalf("param facts = %+v, want one", pf)
	}
	p := pf[0]
	if p.Subject != "DEMO-CAP-6V3" || p.Object != "VDC" || p.Num == nil || *p.Num != 6.3 {
		t.Errorf("param subject/symbol/num = %s/%s/%v, want DEMO-CAP-6V3/VDC/6.3", p.Subject, p.Object, p.Num)
	}
	if p.Conditions != "TA = 25C" {
		t.Errorf("param conditions = %q, want %q", p.Conditions, "TA = 25C")
	}

	// component-on-net(C1, +10V) and (C1, GND)
	on := byRel[RelComponentOnNet]
	if len(on) != 2 {
		t.Fatalf("component-on-net = %+v, want two (C1 on +10V and GND)", on)
	}
	nets := map[string]bool{on[0].Object: true, on[1].Object: true}
	if !nets["+10V"] || !nets["GND"] {
		t.Errorf("component-on-net nets = %v, want +10V and GND", nets)
	}
}

// TestTypesPowerOutFact (WS3-072): the capability flag projects one row on a format that types power
// outputs (KiCad) and zero on one that does not (EDIF) — the queryable twin of the design.types_power_out
// gate, so "is a driver-absence check sound on this design" is answerable from a query.
func TestTypesPowerOutFact(t *testing.T) {
	k := factsByRelation(Facts(NewModel(&ir.Design{SourceFormat: "kicad-sch"})))[RelTypesPowerOut]
	if len(k) != 1 || k[0].Subject != "true" {
		t.Errorf("kicad-sch: want one types_power_out row (true), got %+v", k)
	}
	e := factsByRelation(Facts(NewModel(&ir.Design{SourceFormat: "edif-2.0.0"})))[RelTypesPowerOut]
	if len(e) != 0 {
		t.Errorf("edif: want no types_power_out row, got %+v", e)
	}
}

// TestFeedbackFacts (WS3-067): a feedback-named net projects a feedback fact (the datalog equivalent
// of the test-point rule's exclusion); a plain rail does not.
func TestFeedbackFacts(t *testing.T) {
	d := &ir.Design{Nets: []*ir.Net{
		{Name: "VCC1V2_FB", Prov: &ir.Provenance{SourceFile: "t"}},
		{Name: "VCC", Prov: &ir.Provenance{SourceFile: "t"}},
	}}
	fb := factsByRelation(Facts(NewModel(d)))[RelFeedback]
	if len(fb) != 1 || fb[0].Subject != "VCC1V2_FB" {
		t.Errorf("feedback facts = %+v, want one (VCC1V2_FB)", fb)
	}
}

// TestEsdRatedFacts (WS3-076): component.esd_rated projects a part whose seeded datasheet declares an
// ESD rating at or above the credit floor (the concept esd-protection's Go rule credits, now datalog).
// A below-floor rating and an unseeded part yield no row, and the relation is empty without --params.
func TestEsdRatedFacts(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "U9", Attributes: map[string]string{"MPN": "DEMO-XCVR"}, Prov: &ir.Provenance{SourceFile: "t"}},
		{RefDes: "U8", Attributes: map[string]string{"MPN": "DEMO-WEAK"}, Prov: &ir.Provenance{SourceFile: "t"}},
		{RefDes: "R1", Prov: &ir.Provenance{SourceFile: "t"}}, // no MPN, no spec
	}}
	set := param.ParamSet{
		"DEMO-XCVR": esdSpec("DEMO-XCVR", 8000), // 8 kV, above the 2 kV floor -> rated
		"DEMO-WEAK": esdSpec("DEMO-WEAK", 500),  // below floor -> not credited
	}
	rated := factsByRelation(Facts(NewModelWithParams(d, nil, set)))[RelEsdRated]
	if len(rated) != 1 || rated[0].Subject != "U9" {
		t.Fatalf("component.esd_rated = %+v, want one (U9); DEMO-WEAK below floor and R1 unseeded must not appear", rated)
	}
	if rated[0].Cite == "" {
		t.Error("component.esd_rated fact has no cite; it should point to the datasheet ESD row")
	}
	if got := factsByRelation(Facts(NewModel(d)))[RelEsdRated]; len(got) != 0 {
		t.Errorf("component.esd_rated without --params = %+v, want none (silent by construction)", got)
	}
}

// TestNetBusLikeFacts (WS3-080): net.bus_like projects a shared-distribution net — rail-scale fan-out,
// a ground name, or the global fact — the same predicate the reach walk stops at; a point-to-point net
// does not project.
func TestNetBusLikeFacts(t *testing.T) {
	wide := &ir.Net{Name: "WIDE", Prov: &ir.Provenance{SourceFile: "t"}}
	for i := 0; i < 17; i++ { // > maxWalkFan (16)
		wide.Connections = append(wide.Connections, &ir.Connection{ComponentRef: "U1", PinRef: "p"})
	}
	d := &ir.Design{Nets: []*ir.Net{
		wide,
		{Name: "GND", Prov: &ir.Provenance{SourceFile: "t"}}, // ground name -> bus-like
		{Name: "SIG", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{
			{ComponentRef: "U1", PinRef: "a"}, {ComponentRef: "U2", PinRef: "b"}}}, // 2-pin -> not
	}}
	got := map[string]bool{}
	for _, f := range factsByRelation(Facts(NewModel(d)))[RelNetBusLike] {
		got[f.Subject] = true
	}
	if !got["WIDE"] || !got["GND"] {
		t.Errorf("net.bus_like = %v, want WIDE (fan-out) and GND (ground name)", got)
	}
	if got["SIG"] {
		t.Error("a 2-pin point-to-point net must not be bus_like")
	}
}

// TestFactsAlwaysCited (WS3-004): every fact carries provenance — a fact you cannot cite is not
// verifiable, and verifiability is the point of a provenance-carrying fact base. The param fact
// cites the datasheet document/page; the IR facts cite the source file.
func TestFactsAlwaysCited(t *testing.T) {
	set := param.ParamSet{"DEMO-CAP-6V3": capSpec("DEMO-CAP-6V3", 6.3)}
	facts := Facts(NewModelWithParams(capDesign("+10V", "DEMO-CAP-6V3"), nil, set))
	if len(facts) == 0 {
		t.Fatal("no facts derived")
	}
	for _, f := range facts {
		if f.Cite == "" {
			t.Errorf("fact %s(%s,%s) has no provenance cite", f.Relation, f.Subject, f.Object)
		}
	}
	cite := factsByRelation(facts)[RelParam][0].Cite
	for _, want := range []string{"ACME-CAP Rev C", "page 2", "Ratings"} {
		if !strings.Contains(cite, want) {
			t.Errorf("param cite = %q, missing %q (want the datasheet doc/page/table)", cite, want)
		}
	}
}

// TestFactsRegenerable (WS3-004): the projection is a derived, deterministic view of the Model
// (C8) — regenerating it yields an identical result, so it is safe to recompute rather than
// store as a second authority.
func TestFactsRegenerable(t *testing.T) {
	set := param.ParamSet{"DEMO-CAP-6V3": capSpec("DEMO-CAP-6V3", 6.3)}
	m := NewModelWithParams(capDesign("+10V", "DEMO-CAP-6V3"), nil, set)
	if !reflect.DeepEqual(Facts(m), Facts(m)) {
		t.Error("Facts(m) is not deterministic across calls")
	}
}

// TestBoardFacts (WS1-041): the board tier projects per-net derived facts — the minimum track
// width and via drill (mm), and layer membership — reusing the DRC board fixture. This is what
// makes board geometry queryable through the same fact base, with no query-engine change.
func TestBoardFacts(t *testing.T) {
	byRel := factsByRelation(Facts(NewModelWithBoard(&ir.Design{}, drcBoard())))

	tw := map[string]float64{}
	for _, f := range byRel[RelBoardTrackWidth] {
		if f.Num != nil {
			tw[f.Subject] = *f.Num
		}
	}
	if tw["THIN"] != 0.05 || tw["CLEAN"] != 0.25 {
		t.Errorf("track widths = %v, want THIN=0.05 CLEAN=0.25 (mm, the per-net minimum)", tw)
	}

	vd := map[string]float64{}
	for _, f := range byRel[RelBoardViaDrill] {
		if f.Num != nil {
			vd[f.Subject] = *f.Num
		}
	}
	if vd["SMALLHOLE"] != 0.1 {
		t.Errorf("SMALLHOLE via drill = %v, want 0.1mm", vd["SMALLHOLE"])
	}

	layers := map[string]bool{}
	for _, f := range byRel[RelBoardLayer] {
		if f.Subject == "CLOSE_B" {
			layers[f.Object] = true
		}
	}
	if !layers["F.Cu"] || !layers["B.Cu"] {
		t.Errorf("CLOSE_B layers = %v, want F.Cu and B.Cu", layers)
	}

	for _, rel := range []string{RelBoardTrackWidth, RelBoardViaDrill, RelBoardLayer} {
		for _, f := range byRel[rel] {
			if f.Cite == "" {
				t.Errorf("%s(%s) has no provenance cite", rel, f.Subject)
			}
		}
	}
	// A netlist-only design yields no board facts (silent-by-construction, like the params tier).
	if n := len(factsByRelation(Facts(NewModel(&ir.Design{})))[RelBoardTrackWidth]); n != 0 {
		t.Errorf("board.track_width facts without a board tier = %d, want 0", n)
	}
}

// TestComponentClassAndNetAttrFacts (WS3-074): the class + net-attribute relations project the
// datalog reads a class-quantified rule needs. component.class carries each component's established
// device class and OMITS ClassUnknown (never guessed); net.ground isolates ground-named nets from
// the rail relation (which covers power AND ground); net.external marks read-gap nets. All cited.
func TestComponentClassAndNetAttrFacts(t *testing.T) {
	ext := tnet("XEXT", "Y1.1")
	ext.Attributes = map[string]string{netgraph.AttrExternal: "true"}
	d := &ir.Design{
		Components: []*ir.Component{
			{RefDes: "Y1", Prov: &ir.Provenance{SourceFile: "t"}}, // clock family (bare Y prefix, WS10-015)
			{RefDes: "C1", Prov: &ir.Provenance{SourceFile: "t"}}, // capacitor (C prefix)
			{RefDes: "W1", Prov: &ir.Provenance{SourceFile: "t"}}, // unmapped prefix -> ClassUnknown, omitted
		},
		Nets: []*ir.Net{
			tnet("XIN", "Y1.2", "C1.1"),
			tnet("GND", "C1.2"),
			ext,
		},
	}
	byRel := factsByRelation(Facts(NewModel(d)))

	// component.class: Y1->clock (bare Y is the ambiguous clock family, WS10-015), C1->capacitor; the
	// unclassifiable W1 yields no row.
	class := map[string]string{}
	for _, f := range byRel[RelComponentClass] {
		class[f.Subject] = f.Value
	}
	if class["Y1"] != "clock" || class["C1"] != "capacitor" {
		t.Errorf("component.class = %v, want Y1=clock C1=capacitor", class)
	}
	if _, ok := class["W1"]; ok {
		t.Errorf("component.class included W1 (ClassUnknown must be omitted): %v", class)
	}

	// net.ground: exactly GND (rail would also match power rails, this isolates ground).
	ground := map[string]bool{}
	for _, f := range byRel[RelNetGround] {
		ground[f.Subject] = true
	}
	if len(ground) != 1 || !ground["GND"] {
		t.Errorf("net.ground = %v, want exactly {GND}", ground)
	}

	// net.external: exactly the flagged read-gap net.
	external := map[string]bool{}
	for _, f := range byRel[RelNetExternal] {
		external[f.Subject] = true
	}
	if len(external) != 1 || !external["XEXT"] {
		t.Errorf("net.external = %v, want exactly {XEXT}", external)
	}

	for _, rel := range []string{RelComponentClass, RelNetGround, RelNetExternal} {
		for _, f := range byRel[rel] {
			if f.Cite == "" {
				t.Errorf("%s(%s) has no provenance cite", rel, f.Subject)
			}
		}
	}
}

// TestFactsSilentWithoutDatasheet (WS3-004): a design read without a seeded datasheet set yields
// only the IR relations (net/connection) and no mpn/param facts — the same silent-by-construction
// posture the datasheet rules have; the projection never fabricates a datasheet it does not have.
func TestFactsSilentWithoutDatasheet(t *testing.T) {
	byRel := factsByRelation(Facts(NewModel(capDesign("+10V", ""))))
	if len(byRel[RelParam]) != 0 || len(byRel[RelComponentMPN]) != 0 {
		t.Errorf("param/mpn facts without a seeded set = %d/%d, want 0/0", len(byRel[RelParam]), len(byRel[RelComponentMPN]))
	}
	if len(byRel[RelComponentOnNet]) == 0 || len(byRel[RelNetMaxVoltage]) == 0 {
		t.Error("IR relations (net/connection) should still project from a bare design")
	}
}
