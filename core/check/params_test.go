package check

import (
	"testing"

	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/core/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// supplyDesign builds a one-part design, U1 (part LDO, pin 1 = VDD power_in), with its
// VDD pin on the given net, joined to a spec via the given identity channel. The Available
// param-tier gate tests here share it, and the datasheet rule tests in stdlib/rules/builtin use a
// copy.
func supplyDesign(netName string, viaBomLine bool, mpn string) *ir.Design {
	d := &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{{
			Name: "LDO",
			Pins: []*ir.Pin{{Name: "VDD", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN}},
		}}}},
		Components: []*ir.Component{{
			RefDes:     "U1",
			Sections:   []*ir.ComponentSection{{PartRef: "LDO", LibraryRef: "lib"}},
			Attributes: map[string]string{},
			Prov:       &ir.Provenance{SourceFile: "t"},
		}},
		Nets: []*ir.Net{{
			Name:        netName,
			Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}},
			Prov:        &ir.Provenance{SourceFile: "t"},
		}},
	}
	if mpn != "" {
		if viaBomLine {
			d.Bom = []*ir.BomLine{{RefDes: []string{"U1"}, Mpn: mpn, Manufacturer: "Acme"}}
		} else {
			d.Components[0].Mpn = mpn
		}
	}
	return d
}

// TestUnitVocabulariesAgree is a drift tripwire between the two layers that own a unit vocabulary.
//
// core/classify parses a component's value TEXT off a design and normalizes it to an SI base unit;
// core/param converts a seeded parameter's printed unit to the same base. They are deliberately
// SEPARATE tables, because IEC 60062's RKM code reads M as mega and is case-insensitive on k and u.
// That is correct for a schematic value field, and it inverts three orders of magnitude on a
// printed unit symbol. The datasheet tier also imports nothing from core (C17), so there is no
// import to hold them together.
//
// They must agree on the CANONICAL SPELLING that each normalizes to. A rule comparing a design-side
// resistance against a datasheet-side one compares two unit strings, so a divergence here would make
// every such comparison silently find nothing. This file guards that failure. This test lives in
// core/check because only that package imports both layers.
func TestUnitVocabulariesAgree(t *testing.T) {
	if param.UnitOhm != classify.UnitOhm {
		t.Errorf("ohm symbol diverged: core/param has %q (%U), core/classify has %q (%U)",
			param.UnitOhm, []rune(param.UnitOhm), classify.UnitOhm, []rune(classify.UnitOhm))
	}
	// classify normalizes a design value to a base unit. Each such unit must survive a round trip
	// through the parameter layer unchanged. Otherwise a datasheet row in that unit could never be
	// compared against a component carrying it.
	for _, unit := range []string{classify.UnitOhm, "F", "H", "A", "V"} {
		base, exp, ok := param.BaseUnit(unit)
		if !ok || base != unit || exp != 0 {
			t.Errorf("classify normalizes design values to %q, but param.BaseUnit gives (%q, %d, %v)",
				unit, base, exp, ok)
		}
	}
}

// SeedsAnySymbol answers "does ANY part seed this". That answer decides the verdict, and nobody can
// act on it. UnseededSymbols keeps what that walk discards, per PART, the unit a datasheet author
// works in.
func TestUnseededSymbols(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "U1", Mpn: "ACME-1"},
		{RefDes: "U2", Mpn: "ACME-1"}, // same part, second placement
		{RefDes: "U3", Mpn: "ACME-2"}, // no spec at all
		{RefDes: "R9"},                // no MPN: nothing to name
	}}
	provider := param.ProviderFunc(func(mpn string) *parampb.PartSpec {
		if mpn == "ACME-1" {
			return &parampb.PartSpec{
				Mpn: "ACME-1", Manufacturer: "MakerCo", DeviceClass: "ldo",
				Parameters: []*parampb.Parameter{{Symbol: "VIN"}},
			}
		}
		return nil
	})
	m := NewModel(d, WithParamProvider(provider))

	got := UnseededSymbols(m, []string{"VCC"}, nil)
	if len(got) != 2 {
		t.Fatalf("want one dependency per PART, not per placement; got %d: %+v", len(got), got)
	}
	if got[0].MPN != "ACME-1" || got[1].MPN != "ACME-2" {
		t.Errorf("want deterministic order by mpn, got %q then %q", got[0].MPN, got[1].MPN)
	}
	if got[0].SpecAbsent || got[0].Manufacturer != "MakerCo" {
		t.Errorf("ACME-1 has a spec that lacks VCC: %+v", got[0])
	}
	if !got[1].SpecAbsent || got[1].Manufacturer != "" {
		t.Errorf("ACME-2 has no spec, so the next step is extracting one and the maker is unknown: %+v", got[1])
	}
	for _, dep := range got {
		if dep.MPN == "" {
			t.Error("a dependency naming no part cannot be acted on and must not be emitted")
		}
	}

	// A symbol the part already seeds is not a gap.
	if seeded := UnseededSymbols(m, []string{"VIN"}, nil); len(seeded) != 1 || seeded[0].MPN != "ACME-2" {
		t.Errorf("VIN is seeded on ACME-1, so only ACME-2 should want it; got %+v", seeded)
	}
	// Nothing asked, nothing unmet.
	if none := UnseededSymbols(m, nil, nil); none != nil {
		t.Errorf("no symbols asked, got %+v", none)
	}
}

// Without the class gate a symbol needed by three parts names every part on the design that lacks
// it, and a work list nobody can act on is not an improvement on prose.
func TestUnseededSymbolsRespectsClassGate(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "U1", Mpn: "ACME-1"},
		{RefDes: "U2", Mpn: "ACME-2", DeviceClasses: classify.Tags("crystal")},
	}}
	provider := param.ProviderFunc(func(mpn string) *parampb.PartSpec {
		return &parampb.PartSpec{Mpn: mpn, DeviceClass: map[string]string{"ACME-1": "ldo"}[mpn]}
	})
	m := NewModel(d, WithParamProvider(provider))

	all := UnseededSymbols(m, []string{"VCC"}, nil)
	if len(all) != 2 {
		t.Fatalf("ungated wants both parts, got %+v", all)
	}
	gated := UnseededSymbols(m, []string{"VCC"}, []string{"ldo"})
	if len(gated) != 1 || gated[0].MPN != "ACME-1" {
		t.Errorf("gating on ldo must name only the ldo, got %+v", gated)
	}
}

// railSpec types two terminals as a supply input and a ground, the way a vendor pin table does.
// Neither net name below matches any built-in rail vocabulary, so the datasheet is the only
// evidence.
func railSpec(mpn string) *parampb.PartSpec {
	pin := func(id, name string, f parampb.PinFunction) *parampb.Pin {
		return &parampb.Pin{Id: id, Name: name, Function: f,
			Prov: &parampb.ParamProvenance{DocRef: "ds", Page: 3, TableOrFigure: "Pin Functions", Method: "hand", Confidence: 1}}
	}
	return &parampb.PartSpec{
		Mpn: mpn, Docs: []*parampb.SourceDoc{{Id: "ds", Title: mpn + " Rev A"}},
		Pins: []*parampb.Pin{
			pin("vdd", "VDD", parampb.PinFunction_PIN_FUNCTION_POWER_INPUT),
			pin("vss", "VSS", parampb.PinFunction_PIN_FUNCTION_GROUND),
			pin("io", "IO", parampb.PinFunction_PIN_FUNCTION_INPUT),
		},
	}
}

// houseNamedRailDesign uses a function-first naming convention. The built-in start-anchored
// vocabulary (VCC / VDD / +3V3) matches none of its names. That case made a real board show 13
// rails instead of 91 until its project declared a lexicon.
func houseNamedRailDesign(mpn string) *ir.Design {
	return &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{{Name: "IC", Pins: []*ir.Pin{
			{Name: "VDD", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN},
			{Name: "VSS", Designator: "2"},
			{Name: "IO", Designator: "3", Direction: ir.PinDirection_PIN_DIRECTION_INPUT},
		}}}}},
		Components: []*ir.Component{{RefDes: "U1",
			Sections: []*ir.ComponentSection{{PartRef: "IC", LibraryRef: "lib"}},
			Mpn:      mpn, Prov: &ir.Provenance{SourceFile: "t"}}},
		Nets: []*ir.Net{
			{Name: "PMIC_CORE_3V3", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "t"}},
			{Name: "SYS_RETURN", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "2"}}, Prov: &ir.Provenance{SourceFile: "t"}},
			{Name: "SENSOR_ALERT", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "3"}}, Prov: &ir.Provenance{SourceFile: "t"}},
		},
	}
}

func netNamed(m Model, name string) *ir.Net {
	for _, n := range m.Nets() {
		if n.GetName() == name {
			return n
		}
	}
	return nil
}

// The tier exists because a datasheet classifies rails that a naming convention cannot reach. None of
// these names matches the built-in vocabulary, so without the params tier the design has no rails
// and no ground at all.
func TestDatasheetEvidenceClassifiesRailsANameCannotReach(t *testing.T) {
	mpn := "ACME-IC"
	bare := NewModel(houseNamedRailDesign(mpn))
	if bare.IsRailNet(netNamed(bare, "PMIC_CORE_3V3")) {
		t.Fatal("precondition: the built-in vocabulary must NOT match this name, else the test proves nothing")
	}

	m := NewModel(houseNamedRailDesign(mpn), WithParamProvider(param.ParamSet{mpn: railSpec(mpn)}))
	if !m.IsRailNet(netNamed(m, "PMIC_CORE_3V3")) {
		t.Error("a net feeding a terminal the vendor types POWER_INPUT is a rail")
	}
	if !m.IsGroundNet(netNamed(m, "SYS_RETURN")) {
		t.Error("a net reaching a terminal the vendor types GROUND is ground")
	}
	if m.IsRailNet(netNamed(m, "SENSOR_ALERT")) {
		t.Error("a signal pin evidences nothing about its net; it must not become a rail")
	}
}

// The evidence is recorded as what it is, so a consumer can weigh it rather than only read it.
func TestDatasheetEvidenceRecordsItsSource(t *testing.T) {
	mpn := "ACME-IC"
	m := NewModel(houseNamedRailDesign(mpn), WithParamProvider(param.ParamSet{mpn: railSpec(mpn)}))

	src, ok := NetRoleSource(netNamed(m, "PMIC_CORE_3V3"), ir.Role_ROLE_RAIL, func(string) bool { return false })
	if !ok || src != ir.RoleSource_ROLE_SOURCE_DATASHEET {
		t.Errorf("got (%v, %v), want (DATASHEET, true)", src, ok)
	}
}

// Degrade-safe (C9). With no params tier the design classifies EXACTLY as it did before this pass
// existed. That property makes admitting an evidence tier safe.
func TestDatasheetEvidenceAbsentChangesNothing(t *testing.T) {
	m := NewModel(houseNamedRailDesign("ACME-IC"))
	for _, name := range []string{"PMIC_CORE_3V3", "SYS_RETURN", "SENSOR_ALERT"} {
		if n := netNamed(m, name); len(n.GetRoles()) != 0 {
			t.Errorf("%s: no params means no datasheet roles, got %v", name, n.GetRoles())
		}
	}
}

// Additive only. A role that the NAME established keeps its stronger-or-equal source and is never
// downgraded, and a datasheet can only ever add. Here the name already says rail.
func TestDatasheetEvidenceOnlyAdds(t *testing.T) {
	mpn := "ACME-IC"
	d := houseNamedRailDesign(mpn)
	d.Nets[0].Name = "+3V3" // now the built-in vocabulary matches too
	classify.StampNetRoles(d)
	m := NewModel(d, WithParamProvider(param.ParamSet{mpn: railSpec(mpn)}))

	n := netNamed(m, "+3V3")
	if !m.IsRailNet(n) {
		t.Fatal("the net must still be a rail")
	}
	if got := len(n.GetRoles()); got != 1 {
		t.Errorf("one role established by two tiers stays one role, got %d: %v", got, n.GetRoles())
	}
	// Stronger evidence upgrades the record; the role itself was never at risk.
	if src, _ := NetRoleSource(n, ir.Role_ROLE_RAIL, func(string) bool { return false }); src != ir.RoleSource_ROLE_SOURCE_DATASHEET {
		t.Errorf("datasheet outranks convention for the same role, got %v", src)
	}
}

// Idempotence. Two models over one design must not duplicate roles, since the pass mutates the IR.
func TestDatasheetEvidenceIsIdempotent(t *testing.T) {
	mpn := "ACME-IC"
	d := houseNamedRailDesign(mpn)
	set := param.ParamSet{mpn: railSpec(mpn)}
	NewModel(d, WithParamProvider(set))
	m := NewModel(d, WithParamProvider(set))

	if got := len(netNamed(m, "PMIC_CORE_3V3").GetRoles()); got != 1 {
		t.Errorf("re-running the pass merges rather than appending; got %d roles", got)
	}
}
