package query

import (
	"testing"

	"github.com/panyam/agni/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/datasheet/param"
)

// pinDesign: U1 (part MCU) has a power pin "1" (VDD) alone on net STUB (one connection) and a
// signal pin "2" (IO) on net SHARED with R1 (two connections).
func pinDesign() *ir.Design {
	mcu := &ir.PartType{Name: "MCU", Pins: []*ir.Pin{
		{Name: "VDD", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN},
		{Name: "IO", Designator: "2", Direction: ir.PinDirection_PIN_DIRECTION_INPUT},
	}}
	res := &ir.PartType{Name: "RES", Pins: []*ir.Pin{
		{Name: "~", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_PASSIVE},
	}}
	comp := func(ref, part string) *ir.Component {
		return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"},
			Sections: []*ir.ComponentSection{{PartRef: part, LibraryRef: "lib"}}}
	}
	return &ir.Design{
		Libraries:  []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{mcu, res}}},
		Components: []*ir.Component{comp("U1", "MCU"), comp("R1", "RES")},
		Nets: []*ir.Net{
			{Name: "STUB", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}},
			{Name: "SHARED", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "2"}, {ComponentRef: "R1", PinRef: "1"}}},
		},
	}
}

// The pin relations resolve: role from the pin name, type from the direction, net membership, and
// per-net fan-out.
func TestPinRelationsQuery(t *testing.T) {
	b := NewBase(check.NewModel(pinDesign()))
	q, err := Parse(`pin.role(?r, ?p, ?role), pin.type(?r, ?p, ?etype), pin.net(?r, ?p, ?net) => ?r, ?p, ?role, ?etype, ?net`)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := Naive{}.Eval(q, b)
	if err != nil {
		t.Fatal(err)
	}
	// Only the VDD pin has a derived role (power); IO is RoleUnknown so pin.role omits it.
	if len(rows) != 1 {
		t.Fatalf("want 1 role-bearing pin, got %d: %+v", len(rows), rows)
	}
	got := rows[0].Bind
	if got[Var("r")].S != "U1" || got[Var("p")].S != "1" || got[Var("role")].S != "power" ||
		got[Var("etype")].S != "power_in" || got[Var("net")].S != "STUB" {
		t.Fatalf("unexpected bindings: %+v", got)
	}
}

// TestEsdRatedQuery (WS3-076): the IC-ESD credit expressed as datalog — component.esd_rated joins
// with component-on-net to name the signals a rated transceiver protects. The relation is empty
// without --params, so the query is authored against a params-seeded Base.
func TestEsdRatedQuery(t *testing.T) {
	esd := func(mpn string, volts float64) *parampb.PartSpec {
		v := volts
		return &parampb.PartSpec{Mpn: mpn, Docs: []*parampb.SourceDoc{{Id: "ds", Title: mpn + " Rev A"}},
			Parameters: []*parampb.Parameter{{
				Symbol: "V_ESD", LimitKind: parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX, Unit: "V",
				Value:             &parampb.RangeValue{Max: &v},
				ConditionCoverage: parampb.ConditionCoverage_CONDITION_COVERAGE_UNCONDITIONAL,
				Attributes:        map[string]string{"esd_test_model": "iec"}, // system-level (WS3-077)
				Prov:              &parampb.ParamProvenance{DocRef: "ds", Page: 1},
			}}}
	}
	d := &ir.Design{
		Components: []*ir.Component{
			{RefDes: "U9", Attributes: map[string]string{"MPN": "XCVR"}, Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "U8", Attributes: map[string]string{"MPN": "PLAIN"}, Prov: &ir.Provenance{SourceFile: "t"}},
		},
		Nets: []*ir.Net{
			{Name: "CAN_H", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{{ComponentRef: "U9", PinRef: "1"}}},
			{Name: "GPIO", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{{ComponentRef: "U8", PinRef: "1"}}},
		},
	}
	set := param.ParamSet{"XCVR": esd("XCVR", 8000), "PLAIN": esd("PLAIN", 500)} // only XCVR clears the floor
	b := NewBase(check.NewModelWithParams(d, nil, set))
	q, err := Parse(`component.esd_rated(?r), component-on-net(?r, ?n) => ?n`)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := Naive{}.Eval(q, b)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Bind[Var("n")].S != "CAN_H" {
		t.Fatalf("esd-rated join = %+v, want one net (CAN_H); the below-floor PLAIN part must not credit GPIO", rows)
	}
}

// TestDiagnosticRelationsQuery (WS3-081): the entity-keyed reader diagnostics are queryable and join.
// A ref-des collision on U1, and U1 pin 1 claimed by two nets (a pin-net conflict) -> both relations
// answer, and pin_net_conflict returns one row per net the conflicted pin touches.
func TestDiagnosticRelationsQuery(t *testing.T) {
	d := &ir.Design{
		InputDiagnostics: &ir.InputDiagnostics{RefDesCollisions: []*ir.RefDesCollision{
			{RefDes: "U1", Instances: []*ir.Provenance{{NativeId: "a"}, {NativeId: "b"}}},
		}},
		// U2 (no collision) has pin 1 claimed by both nets -> pin-net conflict. It must be a DIFFERENT
		// component from the collided U1: a duplicate designator legitimately spans nets, so the conflict
		// detector excludes a ref-des that already collided.
		Components: []*ir.Component{
			{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "U2", Prov: &ir.Provenance{SourceFile: "t"}},
		},
		Nets: []*ir.Net{
			{Name: "NETA", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{{ComponentRef: "U2", PinRef: "1"}}},
			{Name: "NETB", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{{ComponentRef: "U2", PinRef: "1"}}},
		},
	}
	b := NewBase(check.NewModel(d))

	rc, err := Naive{}.Eval(MustParse(`ref_des_collision(?r) => ?r`), b)
	if err != nil {
		t.Fatal(err)
	}
	if len(rc) != 1 || rc[0].Bind[Var("r")].S != "U1" {
		t.Fatalf("ref_des_collision = %+v, want one (U1)", rc)
	}

	pc, err := Naive{}.Eval(MustParse(`pin_net_conflict(?r, ?p, ?net) => ?net`), b)
	if err != nil {
		t.Fatal(err)
	}
	nets := map[string]bool{}
	for _, r := range pc {
		nets[r.Bind[Var("net")].S] = true
	}
	if !nets["NETA"] || !nets["NETB"] {
		t.Fatalf("pin_net_conflict nets = %v, want NETA and NETB (a pin on two nets)", nets)
	}
}

// param.prov exposes a datasheet value's citation as a query relation, and a component-subject
// datalog rule that declares ParamSymbol carries the FULL citation (doc/page/section/method/
// confidence) onto its findings via check.DatasheetProvFor — the datalog analogue of the built-in
// datasheet rules' dual provenance (WS10-010). Both need a params-seeded Model.
func TestParamProvRelationAndFindingAttach(t *testing.T) {
	iout := 1.0
	spec := &parampb.PartSpec{
		Mpn:  "BUCKPART",
		Docs: []*parampb.SourceDoc{{Id: "snas870b", Title: "LMR60410-Q1 Buck (SNAS870B Rev. B)"}},
		Parameters: []*parampb.Parameter{{
			Symbol: "IOUT", LimitKind: parampb.LimitKind_LIMIT_KIND_RECOMMENDED_OPERATING, Unit: "A",
			Value: &parampb.RangeValue{Max: &iout},
			Prov: &parampb.ParamProvenance{
				DocRef: "snas870b", Page: 5, TableOrFigure: "6.3 Recommended Operating Conditions",
				Method: "hand", Confidence: 1.0,
			},
		}},
	}
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "U1", Attributes: map[string]string{"MPN": "BUCKPART"}, Prov: &ir.Provenance{SourceFile: "t"}}},
		Nets:       []*ir.Net{{Name: "V1", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}}},
	}
	m := check.NewModelWithParams(d, nil, param.ParamSet{"BUCKPART": spec})

	// (1) the relation surfaces the citation columns (doc title / page / section).
	rows, err := Naive{}.Eval(MustParse(`param.prov(?mpn, ?sym, ?doc, ?page, ?section) => ?doc, ?page, ?section`), NewBase(m))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 param.prov row, got %d: %+v", len(rows), rows)
	}
	b := rows[0].Bind
	if b[Var("doc")].S != "LMR60410-Q1 Buck (SNAS870B Rev. B)" || b[Var("section")].S != "6.3 Recommended Operating Conditions" {
		t.Fatalf("citation columns wrong: %+v", b)
	}
	if b[Var("page")].Num == nil || *b[Var("page")].Num != 5 {
		t.Fatalf("page not numeric 5: %+v", b[Var("page")])
	}

	// (2) a datalog rule with ParamSymbol attaches the full citation, INCLUDING confidence — the field
	// the report leans on to flag a value that should be verified before it is trusted.
	rule := RuleFromQuery(FindingQuery{
		Rule:        check.Rule{Name: "iout-check", Severity: "warning"},
		Query:       MustParse(`component.mpn(?r, ?m), param(?m, "IOUT", ?v), ?v < 6 => ?r`),
		Kind:        check.KindComponent,
		SubjectVar:  "r",
		Message:     "{r}: IOUT below requirement",
		ParamSymbol: "IOUT",
	})
	fs := rule.Eval(m)
	if len(fs) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(fs), fs)
	}
	dp := fs[0].DatasheetProv
	if dp == nil {
		t.Fatal("DatasheetProv not attached to a ParamSymbol datalog finding")
	}
	if dp.Doc != "LMR60410-Q1 Buck (SNAS870B Rev. B)" || dp.DocRef != "snas870b" || dp.Page != 5 ||
		dp.Section != "6.3 Recommended Operating Conditions" || dp.Method != "hand" || dp.Confidence != 1.0 {
		t.Fatalf("citation not fully attached: %+v", dp)
	}

	// (3) without ParamSymbol, no citation is attached (the opt-in gates it).
	plain := RuleFromQuery(FindingQuery{
		Rule:       check.Rule{Name: "iout-plain", Severity: "warning"},
		Query:      MustParse(`component.mpn(?r, ?m), param(?m, "IOUT", ?v), ?v < 6 => ?r`),
		Kind:       check.KindComponent,
		SubjectVar: "r",
		Message:    "{r}",
	})
	if pf := plain.Eval(m); len(pf) != 1 || pf[0].DatasheetProv != nil {
		t.Fatalf("no ParamSymbol should mean no citation: %+v", pf)
	}
}

// RuleFromQuery turns a datalog goal into a check.Rule whose every answer row is a Finding, with
// the subject's provenance resolved and the message template filled from the row.
func TestRuleFromQuery(t *testing.T) {
	rule := RuleFromQuery(FindingQuery{
		Rule: check.Rule{Name: "pin-on-stub-net", Severity: "warning"},
		// A pin alone on its net (fan-out < 2). Reads is left empty to exercise auto-derivation.
		Query:      MustParse(`pin.net(?ref, ?pin, ?net), net.pin_count(?net, ?c), ?c < 2 => ?ref, ?pin, ?net`),
		Kind:       check.KindPin,
		SubjectVar: "ref",
		PinVar:     "pin",
		Message:    "pin {pin} sits alone on net {net}",
	})
	fs := rule.Eval(check.NewModel(pinDesign()))
	if len(fs) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(fs), fs)
	}
	f := fs[0]
	if f.Kind != check.KindPin || f.Subject != "U1" || f.Pin != "1" {
		t.Fatalf("wrong subject/pin: %+v", f)
	}
	if f.Message != "pin 1 sits alone on net STUB" {
		t.Fatalf("template not filled: %q", f.Message)
	}
	if f.Prov == nil || f.Prov.SourceFile != "t" {
		t.Fatalf("provenance not resolved from the model: %+v", f.Prov)
	}
}
