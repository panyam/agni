package relations

import (
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// factsByRelation indexes a projection by relation name for the assertions below.
func factsByRelation(fs []query.FactRow) map[string][]query.FactRow {
	out := map[string][]query.FactRow{}
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
	m := check.NewModelWithParams(supplyDesign("+5V", false, "ACME-33"), nil, set)
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
	m := check.NewModelWithParams(capDesign("+10V", "DEMO-CAP-6V3"), nil, set)
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
	k := factsByRelation(Facts(check.NewModel(&ir.Design{SourceFormat: "kicad-sch"})))[RelTypesPowerOut]
	if len(k) != 1 || k[0].Subject != "true" {
		t.Errorf("kicad-sch: want one types_power_out row (true), got %+v", k)
	}
	e := factsByRelation(Facts(check.NewModel(&ir.Design{SourceFormat: "edif-2.0.0"})))[RelTypesPowerOut]
	if len(e) != 0 {
		t.Errorf("edif: want no types_power_out row, got %+v", e)
	}
}

// TestExternalSignalNetFacts (WS3-061): the relation projects check.ExternalSignalNet, so a datalog
// ESD check selects the same nets the Go rules do. The exclusions are the assertions that matter: a
// rail and a ground net reaching the same connector must NOT appear, because a dropped guard here is
// a false FAIL on a net that was never an ESD question.
func TestExternalSignalNetFacts(t *testing.T) {
	d := &ir.Design{
		Components: []*ir.Component{
			{RefDes: "J1", Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "t"}},
		},
		Nets: []*ir.Net{
			{Name: "BUS_CANH", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{
				{ComponentRef: "J1", PinRef: "1"}, {ComponentRef: "U1", PinRef: "1"}}},
			{Name: "GND", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{
				{ComponentRef: "J1", PinRef: "2"}, {ComponentRef: "U1", PinRef: "2"}}},
			{Name: "+12V", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{
				{ComponentRef: "J1", PinRef: "3"}, {ComponentRef: "U1", PinRef: "3"}}},
			{Name: "INTERNAL", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{
				{ComponentRef: "U1", PinRef: "4"}}},
		},
	}
	got := map[string]bool{}
	for _, f := range factsByRelation(Facts(check.NewModel(d)))[RelExternalSignalNet] {
		got[f.Subject] = true
		if f.Cite == "" {
			t.Errorf("external_signal_net(%s) has no provenance cite", f.Subject)
		}
	}
	if !got["BUS_CANH"] {
		t.Errorf("want BUS_CANH (a signal net on a connector), got %v", got)
	}
	for _, excluded := range []string{"GND", "+12V", "INTERNAL"} {
		if got[excluded] {
			t.Errorf("%s must not be in the ESD scope: %v", excluded, got)
		}
	}
}

// TestNetClassFacts (WS3-105): net.netclass projects the TOOL-assigned class verbatim, one row per
// classed net, and leaves an unclassed net out (so `not net.netclass(?n, ?_)` reads as unclassed).
// has_netclass is the design-level marker that separates "no net is in class X" from "this design
// assigns no classes", the distinction a netclass-scoped rule needs to avoid reading as a pass.
func TestNetClassFacts(t *testing.T) {
	d := &ir.Design{Nets: []*ir.Net{
		{Name: "USB_D+", NetClasses: []string{"HighSpeed"}, Prov: &ir.Provenance{SourceFile: "t"}},
		{Name: "VBUS", NetClasses: []string{"Power"}, Prov: &ir.Provenance{SourceFile: "t"}},
		{Name: "SCL", Prov: &ir.Provenance{SourceFile: "t"}},
	}}
	byRel := factsByRelation(Facts(check.NewModel(d)))

	got := map[string]string{}
	for _, f := range byRel[RelNetNetClass] {
		got[f.Subject] = f.Value
		if f.Cite == "" {
			t.Errorf("net.netclass(%s) has no provenance cite", f.Subject)
		}
	}
	want := map[string]string{"USB_D+": "HighSpeed", "VBUS": "Power"}
	if len(got) != len(want) || got["USB_D+"] != want["USB_D+"] || got["VBUS"] != want["VBUS"] {
		t.Errorf("net.netclass = %v, want %v (SCL is unclassed and must not appear)", got, want)
	}

	if mk := byRel[RelHasNetClass]; len(mk) != 1 || mk[0].Subject != "true" {
		t.Errorf("want one has_netclass row (true), got %+v", mk)
	}
}

// TestNetClassFactsFanOut (WS1-050): membership is a set, so the projection is 1:many — a net in
// two classes emits one row per class and `?net` is NOT unique. This is what a rule joining on ?net
// must expect; the relation shape is unchanged (it was already {FieldSubject, FieldValue}), only
// the arity is.
func TestNetClassFactsFanOut(t *testing.T) {
	d := &ir.Design{Nets: []*ir.Net{
		{Name: "VBUS", NetClasses: []string{"HighCurrent", "Power"}, Prov: &ir.Provenance{SourceFile: "t"}},
		{Name: "SCL", Prov: &ir.Provenance{SourceFile: "t"}},
	}}
	rows := factsByRelation(Facts(check.NewModel(d)))[RelNetNetClass]
	if len(rows) != 2 {
		t.Fatalf("want 2 net.netclass rows for a two-class net, got %d: %+v", len(rows), rows)
	}
	got := []string{}
	for _, r := range rows {
		if r.Subject != "VBUS" {
			t.Errorf("unexpected subject %q (SCL is unclassed and must not appear)", r.Subject)
		}
		got = append(got, r.Value)
	}
	sort.Strings(got)
	if want := []string{"HighCurrent", "Power"}; !slices.Equal(got, want) {
		t.Errorf("net.netclass values = %v, want %v", got, want)
	}
}

// TestNetClassFactsAbsent (WS3-105): a design whose source carries no net classes — every format but
// a KiCad project read, and a KiCad project that declares none — projects neither the relation nor
// the marker. The empty marker is the signal a class-scoped rule gates on; without it an empty
// net.netclass join is indistinguishable from a clean pass.
func TestNetClassFactsAbsent(t *testing.T) {
	d := &ir.Design{SourceFormat: "edif-2.0.0", Nets: []*ir.Net{
		{Name: "USB_D+", Prov: &ir.Provenance{SourceFile: "t"}},
	}}
	byRel := factsByRelation(Facts(check.NewModel(d)))
	if n := byRel[RelNetNetClass]; len(n) != 0 {
		t.Errorf("want no net.netclass rows on a classless design, got %+v", n)
	}
	if mk := byRel[RelHasNetClass]; len(mk) != 0 {
		t.Errorf("want no has_netclass row on a classless design, got %+v", mk)
	}
}

// TestFeedbackFacts (WS3-067): a feedback-named net projects a feedback fact (the datalog equivalent
// of the test-point rule's exclusion); a plain rail does not.
func TestFeedbackFacts(t *testing.T) {
	d := &ir.Design{Nets: []*ir.Net{
		{Name: "VCC1V2_FB", Prov: &ir.Provenance{SourceFile: "t"}},
		{Name: "VCC", Prov: &ir.Provenance{SourceFile: "t"}},
	}}
	fb := factsByRelation(Facts(check.NewModel(d)))[RelFeedback]
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
	rated := factsByRelation(Facts(check.NewModelWithParams(d, nil, set)))[RelEsdRated]
	if len(rated) != 1 || rated[0].Subject != "U9" {
		t.Fatalf("component.esd_rated = %+v, want one (U9); DEMO-WEAK below floor and R1 unseeded must not appear", rated)
	}
	if rated[0].Cite == "" {
		t.Error("component.esd_rated fact has no cite; it should point to the datasheet ESD row")
	}
	if got := factsByRelation(Facts(check.NewModel(d)))[RelEsdRated]; len(got) != 0 {
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
	for _, f := range factsByRelation(Facts(check.NewModel(d)))[RelNetBusLike] {
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
	facts := Facts(check.NewModelWithParams(capDesign("+10V", "DEMO-CAP-6V3"), nil, set))
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
	m := check.NewModelWithParams(capDesign("+10V", "DEMO-CAP-6V3"), nil, set)
	if !reflect.DeepEqual(Facts(m), Facts(m)) {
		t.Error("Facts(m) is not deterministic across calls")
	}
}

// TestBoardFacts (WS1-041): the board tier projects per-net derived facts — the minimum track
// width and via drill (mm), and layer membership — reusing the DRC board fixture. This is what
// makes board geometry queryable through the same fact base, with no query-engine change.
func TestBoardFacts(t *testing.T) {
	byRel := factsByRelation(Facts(check.NewModelWithBoard(&ir.Design{}, drcBoard())))

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
	if n := len(factsByRelation(Facts(check.NewModel(&ir.Design{})))[RelBoardTrackWidth]); n != 0 {
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
	byRel := factsByRelation(Facts(check.NewModel(d)))

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
	byRel := factsByRelation(Facts(check.NewModel(capDesign("+10V", ""))))
	if len(byRel[RelParam]) != 0 || len(byRel[RelComponentMPN]) != 0 {
		t.Errorf("param/mpn facts without a seeded set = %d/%d, want 0/0", len(byRel[RelParam]), len(byRel[RelComponentMPN]))
	}
	if len(byRel[RelComponentOnNet]) == 0 || len(byRel[RelNetMaxVoltage]) == 0 {
		t.Error("IR relations (net/connection) should still project from a bare design")
	}
}

// TestNetBiasAndACCoupledFacts (WS3-088): the two derived net properties project as relations, so the
// intent rules that compare them against a declaration and an engineer's ad-hoc query read one
// definition rather than two.
//
// The assertions that matter are the negative ones. A DECOUPLING cap must not read as AC-coupled, or
// nearly every net on a board would; and a divider must not read as biased, since it holds neither
// rail.
func TestNetBiasAndACCoupledFacts(t *testing.T) {
	net := func(name string, conns ...string) *ir.Net {
		n := &ir.Net{Name: name, Prov: &ir.Provenance{SourceFile: "t"}}
		for _, c := range conns {
			n.Connections = append(n.Connections, &ir.Connection{ComponentRef: c, PinRef: "1"})
		}
		return n
	}
	d := &ir.Design{
		Components: []*ir.Component{
			{RefDes: "R1", Prov: &ir.Provenance{SourceFile: "t"}}, // PULLED_UP -> +3V3
			{RefDes: "R2", Prov: &ir.Provenance{SourceFile: "t"}}, // DIVIDED  -> +3V3
			{RefDes: "R3", Prov: &ir.Provenance{SourceFile: "t"}}, // DIVIDED  -> GND
			{RefDes: "C1", Prov: &ir.Provenance{SourceFile: "t"}}, // COUPLED  -> FAR (a signal)
			{RefDes: "C2", Prov: &ir.Provenance{SourceFile: "t"}}, // BYPASSED -> GND
		},
		Nets: []*ir.Net{
			net("PULLED_UP", "R1"), net("DIVIDED", "R2", "R3"),
			net("COUPLED", "C1"), net("BYPASSED", "C2"), net("FAR", "C1"),
			net("+3V3", "R1", "R2"), net("GND", "R3", "C2"),
		},
	}
	byRel := factsByRelation(Facts(check.NewModel(d)))

	bias := map[string]string{}
	for _, f := range byRel[RelNetBias] {
		bias[f.Subject] = f.Value
	}
	if bias["PULLED_UP"] != "high" {
		t.Errorf("net.bias(PULLED_UP) = %q, want high", bias["PULLED_UP"])
	}
	if lv, ok := bias["DIVIDED"]; ok {
		t.Errorf("a divider holds neither rail, want no row: got %q", lv)
	}

	coupled := map[string]bool{}
	for _, f := range byRel[RelNetACCoupled] {
		coupled[f.Subject] = true
	}
	if !coupled["COUPLED"] {
		t.Errorf("a cap to another signal is coupling: %v", coupled)
	}
	if coupled["BYPASSED"] {
		t.Errorf("a cap to GND decouples, it does not couple: %v", coupled)
	}

	// Neither property is meaningful about a supply net, and BOTH answered from the wrong end before
	// this guard existed: the rail read as biased high (a pull-up connects it to the line it pulls)
	// and GND read as AC-coupled (a crystal load cap puts a signal on its far side). Found by running
	// the relations on a real board, not by a fixture — excluding the far side was never enough, the
	// SUBJECT has to be a signal too.
	for _, supply := range []string{"+3V3", "GND"} {
		if lv, ok := bias[supply]; ok {
			t.Errorf("net.bias(%s) = %q: a supply net is not held at a level, it IS the level", supply, lv)
		}
		if coupled[supply] {
			t.Errorf("net.ac_coupled(%s): a supply net is not a coupled signal", supply)
		}
	}
}

// netClassDef builds an ir.Constraint the way kicad.AnnotateNetClassDefs does. Only stated params
// are set, because absent-vs-zero is the distinction the cascade turns on.
func netClassDef(name string, priority int, params map[string]string) *ir.Constraint {
	p := map[string]string{"priority": strconv.Itoa(priority)}
	for k, v := range params {
		p[k] = v
	}
	return &ir.Constraint{Name: name, Kind: "netclass", Params: p}
}

// TestNetClassDefCascade (WS3-111) is the load-bearing test of the declared-vs-actual story. A net in
// several classes does NOT take one class's values wholesale: KiCad fills each constraint from the
// highest-priority class that states THAT constraint, with the default class last. Getting this
// wrong produces confident, wrong findings, which is why the per-net relation exists at all.
func TestNetClassDefCascade(t *testing.T) {
	d := &ir.Design{
		Constraints: []*ir.Constraint{
			// HighSpeed outranks Power but states no track width, so track width must fall through.
			netClassDef("HighSpeed", 1, map[string]string{"clearance": "0.15"}),
			netClassDef("Power", 5, map[string]string{"track_width": "0.8"}),
			defaultNetClassDef("Default", map[string]string{"track_width": "0.25", "via_drill": "0.3"}),
		},
		Nets: []*ir.Net{
			{Name: "VBUS", NetClasses: []string{"HighSpeed", "Power"}, Prov: &ir.Provenance{SourceFile: "t"}},
			{Name: "PLAIN", NetClasses: []string{"Default"}, Prov: &ir.Provenance{SourceFile: "t"}},
			{Name: "UNCLASSED", Prov: &ir.Provenance{SourceFile: "t"}},
		},
	}
	byRel := factsByRelation(Facts(check.NewModel(d)))

	got := map[string]float64{}
	cite := map[string]string{}
	for _, f := range byRel[RelNetDeclaredTrackWidth] {
		if f.Num == nil {
			t.Fatalf("declared track width for %q has no Num", f.Subject)
		}
		got[f.Subject] = *f.Num
		cite[f.Subject] = f.Cite
	}
	// VBUS is in HighSpeed (priority 1) and Power (5). HighSpeed states no track width, so the
	// value cascades to Power — NOT to Default, and NOT to "HighSpeed states nothing so give up".
	if got["VBUS"] != 0.8 {
		t.Errorf("VBUS declared track width = %v, want 0.8 (cascades past HighSpeed, which states none)", got["VBUS"])
	}
	if cite["VBUS"] != "net_settings:Power" {
		t.Errorf("VBUS cite = %q, want net_settings:Power (a finding must name the class that set the limit)", cite["VBUS"])
	}
	if got["PLAIN"] != 0.25 {
		t.Errorf("PLAIN declared track width = %v, want 0.25", got["PLAIN"])
	}
	// The Default class is not just the lowest-priority one: KiCad hands it to a net that is in NO
	// class, so an unclassed net still has a limit the tool would enforce.
	if got["UNCLASSED"] != 0.25 {
		t.Errorf("UNCLASSED declared track width = %v, want 0.25 (Default applies to every net)", got["UNCLASSED"])
	}
	// One row per net per quantity: the cascade resolved, never one row per class.
	if n := len(byRel[RelNetDeclaredTrackWidth]); n != 3 {
		t.Errorf("declared track width rows = %d, want 3 (resolved per NET, not per class)", n)
	}
	// via_drill is stated only by Default, which fills it for EVERY net including VBUS, whose own
	// two classes state none. This is the half a memberships-only cascade would silently miss.
	if n := len(byRel[RelNetDeclaredViaDrill]); n != 3 {
		t.Errorf("declared via drill rows = %d, want 3 (Default fills gaps for every net)", n)
	}
	// The raw per-class rows are still projected, and they are 1:many by class.
	if n := len(byRel[RelNetClassTrackWidth]); n != 2 {
		t.Errorf("netclass.track_width rows = %d, want 2 (Power and Default state one; HighSpeed does not)", n)
	}
	if mk := byRel[RelHasNetClassDefs]; len(mk) != 1 {
		t.Errorf("want one has_netclass_defs row, got %+v", mk)
	}
}

// TestNetClassDefAbsent (WS3-111): a design with no class definitions projects neither the per-class
// rows, the cascaded rows, nor the marker. Without the marker a declared-vs-actual rule cannot tell
// "nothing to compare" from "everything conformed".
func TestNetClassDefAbsent(t *testing.T) {
	d := &ir.Design{Nets: []*ir.Net{
		// Membership WITHOUT definitions: net_settings allows it, and it is the case that would
		// otherwise read as a clean pass over zero comparisons.
		{Name: "VBUS", NetClasses: []string{"Power"}, Prov: &ir.Provenance{SourceFile: "t"}},
	}}
	byRel := factsByRelation(Facts(check.NewModel(d)))
	for _, rel := range []string{RelNetClassTrackWidth, RelNetDeclaredTrackWidth, RelHasNetClassDefs} {
		if rows := byRel[rel]; len(rows) != 0 {
			t.Errorf("%s on a design with membership but no definitions = %+v, want empty", rel, rows)
		}
	}
	// has_netclass still fires: membership exists. The two markers are independent on purpose.
	if mk := byRel[RelHasNetClass]; len(mk) != 1 {
		t.Errorf("has_netclass = %+v, want one row (membership is present even with no definitions)", mk)
	}
}

// defaultNetClassDef builds the DEFAULT class, which carries KiCad's max-int priority and the flag
// marking it as the class every net inherits from.
func defaultNetClassDef(name string, params map[string]string) *ir.Constraint {
	c := netClassDef(name, 2147483647, params)
	c.Params["is_default"] = "true"
	return c
}

// TestNetClassDefCascadeHonoursPriority exercises the cascade ORDER specifically. The cascade test
// above cannot: there, only one of the net's classes stated a track width, so any ordering produced
// the same answer. Here two classes state one and disagree, and the class names are chosen so
// ALPHABETICAL order is the reverse of priority order — membership arrives alphabetically sorted
// (WS1-050), so a projector that forgot to sort would take the wrong class and look correct.
func TestNetClassDefCascadeHonoursPriority(t *testing.T) {
	d := &ir.Design{
		Constraints: []*ir.Constraint{
			netClassDef("Zeta", 1, map[string]string{"track_width": "0.15"}),  // wins on priority
			netClassDef("Alpha", 9, map[string]string{"track_width": "0.90"}), // sorts first, loses
		},
		Nets: []*ir.Net{{Name: "SIG", NetClasses: []string{"Alpha", "Zeta"}, Prov: &ir.Provenance{SourceFile: "t"}}},
	}
	rows := factsByRelation(Facts(check.NewModel(d)))[RelNetDeclaredTrackWidth]
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want exactly 1 (the cascade resolves to one value)", rows)
	}
	if rows[0].Num == nil || *rows[0].Num != 0.15 {
		t.Errorf("declared track width = %v, want 0.15 from Zeta (priority 1), not 0.9 from Alpha", rows[0].Num)
	}
	if rows[0].Cite != "net_settings:Zeta" {
		t.Errorf("cite = %q, want net_settings:Zeta", rows[0].Cite)
	}
}
