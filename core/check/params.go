package check

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/panyam/agni/core/classify"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/datasheet/param"
)

// The params tier of the Model (WS10-003): the validation join between design
// components and seeded datasheet PartSpecs (agni.v1.param). Same posture as the
// board tier: rules read the joined spec through the Model, never the raw set, and a
// model built without a seeded set (NewModel, NewModelWithBoard) yields nil for every
// component, so datasheet-backed rules are silent by construction — skip, never
// false-pass. Catalog-level gating is Available's "param" read-prefix rule.
//
// The join key is part identity: ir.BomLine (mpn/manufacturer, matched by ref_des)
// when the design carries a BOM, else the component's MPN attribute (the KiCad reader
// carries the MPN/Manufacturer symbol properties into attributes). MPN matching is
// case-insensitive; nothing fuzzier, on purpose (param.ParamSet.Lookup).

// NewModelWithParams builds the default Model with the board tier (bg may be nil) and
// the params tier (specs may be nil) attached. specs is any param.ParamProvider — a
// directory-loaded ParamSet, an in-memory mock, or a datasheet service — so the datasheet
// source is pluggable without touching the model or any rule. NewModel and NewModelWithBoard
// remain the narrower constructors; every existing caller is unchanged (a ParamSet satisfies
// ParamProvider).
func NewModelWithParams(d *ir.Design, bg *geom.BoardGeometry, specs param.ParamProvider, opts ...ModelOption) Model {
	m := NewModelWithBoard(d, bg, opts...).(*irModel)
	m.specs = specs
	m.mpn = map[string]string{}
	for _, line := range d.Bom {
		if line.Mpn == "" {
			continue
		}
		for _, ref := range line.RefDes {
			m.mpn[ref] = line.Mpn
		}
	}
	for _, c := range d.Components {
		if _, ok := m.mpn[c.RefDes]; ok {
			continue
		}
		if v := c.Attributes["MPN"]; v != "" {
			m.mpn[c.RefDes] = v
		} else if v := c.Attributes["mpn"]; v != "" {
			m.mpn[c.RefDes] = v
		}
	}
	m.enrichClassesFromParams()
	return m
}

// enrichClassesFromParams merges each component's datasheet-declared device class into its
// device_classes SET (WS10-013 Phase 2). The datasheet is the authoritative class evidence — a smart
// high-side switch IS an eFuse because its spec says so — but the mpn/classSet maps only exist once a
// params tier is attached, so this runs at model-build time (here), AFTER the ingestion classify pass
// stamped the keyword-derived set. The merge is ADDITIVE: the datasheet class (and its family tags via
// classify.ClassesOf) is added as a membership tag, so HasClass and component.class answer from it,
// but the most-specific ComponentClass is left keyword-derived — a datasheet class is not promoted
// over the ref-des/description class, to avoid perturbing the existing ComponentClass equality sites.
// Degrade-safe (C9): a component with no seeded spec, or a spec with no device_class, keeps exactly its
// keyword-derived set, so a design read without --params classifies identically.
func (m *irModel) enrichClassesFromParams() {
	for _, c := range m.d.Components {
		spec := m.PartSpec(c.RefDes)
		if spec == nil || spec.GetDeviceClass() == "" {
			continue
		}
		// Normalize the free-form vendor device_class ("ceramic resonator", "SPXO") to a canonical class
		// FIRST (WS10-015), so a spelling variant reaches the same family tag the keyword path produces
		// instead of landing bare; an unknown-but-meaningful value still passes through (identity).
		for _, tag := range classify.ClassesOf(classify.NormalizeDeviceClass(spec.GetDeviceClass())) {
			cl := ComponentClass(tag)
			if !slices.Contains(m.classSet[c.RefDes], cl) {
				m.classSet[c.RefDes] = append(m.classSet[c.RefDes], cl)
			}
		}
	}
}

// ComponentMPN returns the design-side part identity for a component: the BomLine MPN
// when a BOM covers the ref_des, else the component's MPN attribute, else "". This is
// the design half of the join key; it never guesses (no Value-field parsing).
func (m *irModel) ComponentMPN(refDes string) string { return m.mpn[refDes] }

// PartSpec returns the seeded datasheet spec joined to a component, or nil when the
// component has no MPN, the MPN is unseeded, or the model was built without a provider.
// Rules treat nil as skip, never as pass. The nil-provider guard matters because specs is
// now an interface: a model built by NewModel / NewModelWithBoard leaves it a nil interface,
// which would panic on a method call (a nil ParamSet map would not).
func (m *irModel) PartSpec(refDes string) *parampb.PartSpec {
	if m.specs == nil {
		return nil
	}
	return m.specs.Lookup(m.mpn[refDes])
}

// supplySymbols is the per-corpus alias map for the "supply voltage" concept: the
// vendor symbols an absolute-maximum supply rating prints under, matched after
// normalization (upper-case, spaces stripped — producers split subscripts). This
// lookup lives here in the model layer so no rule text ever hardcodes a vendor
// spelling (docs/20, comparison semantics). It grows per corpus; WS10-004 replaces it
// with the real ontology.
var supplySymbols = map[string]bool{
	"VIN": true, "VDD": true, "VCC": true, "VDDA": true,
	"VBAT": true, "VSUP": true, "V+": true,
}

// fetBreakdownSymbols is the alias set for a MOSFET's drain-source breakdown voltage: the symbols
// datasheets print it under. Same posture as supplySymbols — vendor spelling lives in the model layer,
// never in rule text (docs/20). Deliberately excludes VGSS (the GATE-source rating), which is a
// different limit on a different pair of terminals and is typically much lower; comparing a rail
// against the wrong one of the two would misreport which rating a design violates.
var fetBreakdownSymbols = map[string]bool{
	"VDSS": true, "VDS": true, "BVDSS": true, "V(BR)DSS": true,
}

// FetBreakdownLimits selects the machine-comparable drain-source breakdown rows of a spec: symbol in
// the breakdown alias set, kind ABSOLUTE_MAX, unit exactly "V", a max bound present, and the docs/20
// comparison gates. Breakdown IS an absolute maximum on a real datasheet (it is the voltage past which
// the part stops being a switch), so unlike OutputVoltageLimits the kind is constrained.
//
// The third member of the connection-aware extractor family: SupplyAbsMaxLimits reads what a part can
// WITHSTAND on its supply, OutputVoltageLimits what a part DELIVERS, and this what a switch can BLOCK.
func FetBreakdownLimits(spec *parampb.PartSpec) []*parampb.Parameter {
	var out []*parampb.Parameter
	for _, p := range spec.Parameters {
		sym := strings.ToUpper(strings.ReplaceAll(p.Symbol, " ", ""))
		if !fetBreakdownSymbols[sym] ||
			p.LimitKind != parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX ||
			p.Unit != "V" || p.Value == nil || p.Value.Max == nil ||
			param.UnderSpecified(p) || !param.MachineComparable(p) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// outputSymbols is the alias set for a regulator's OUTPUT voltage: the symbols datasheets print it
// under. Same posture as supplySymbols — the vendor spelling lives in the model layer, never in rule
// text (docs/20). Deliberately narrow: these are outputs a downstream part is fed BY, so a symbol
// that could mean an input must not be in here or a connection-aware rule would compare the wrong
// end of the part against its neighbour.
var outputSymbols = map[string]bool{
	"VOUT": true, "VO": true, "VOUT1": true, "VOUT2": true, "VOUTA": true, "VOUTB": true,
}

// OutputVoltageLimits selects the machine-comparable OUTPUT-voltage rows of a spec: symbol in the
// output alias set, unit exactly "V", a max bound present, and the docs/20 comparison gates. The
// limit KIND is deliberately not constrained: a regulator states its output as a recommended-operating
// or characteristic row, not an absolute maximum, so filtering to one kind would find nothing on a
// real spec. The MAX is what a downstream part is exposed to, which is the number a compatibility
// check needs.
//
// The counterpart to SupplyAbsMaxLimits: that one reads what a part can WITHSTAND, this reads what a
// part DELIVERS. A connection-aware rule joins one of each across the net between two parts (WS3-028).
func OutputVoltageLimits(spec *parampb.PartSpec) []*parampb.Parameter {
	var out []*parampb.Parameter
	for _, p := range spec.Parameters {
		sym := strings.ToUpper(strings.ReplaceAll(p.Symbol, " ", ""))
		if !outputSymbols[sym] || p.Unit != "V" || p.Value == nil || p.Value.Max == nil ||
			param.UnderSpecified(p) || !param.MachineComparable(p) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// SupplyAbsMaxLimits selects the machine-comparable absolute-maximum supply-voltage
// rows of a spec: symbol in the supply alias set, kind ABSOLUTE_MAX, unit exactly "V"
// (unlike units are under-specified for comparison until WS10-004 — never converted),
// a max bound present, conditions asserted complete and structured
// (param.MachineComparable). Rows failing any gate are skipped, not coerced.
// esdSymbols is the alias set for a part's ESD-tolerance rating: the symbols datasheets print it
// under (the ESD-gun / handling models). Same posture as supplySymbols — the vendor spelling lives in
// the model layer, never in rule text (docs/20).
var esdSymbols = map[string]bool{
	"VESD": true, "V(ESD)": true, "ESD": true,
	"VESDHBM": true, "ESDHBM": true, "VESDCDM": true, "ESDCDM": true,
	"VESDIEC": true, "ESDIEC": true, "VESDCONTACT": true, "VESDAIR": true,
}

// icEsdFloorVolts is the minimum declared ESD tolerance that credits a connector-facing signal as
// IC-protected (WS3-073). Conservative; automotive bus transceivers rate far higher (IEC ±8kV). Two
// refinements are deliberate follow-ups: crediting only a SYSTEM-level test model (IEC 61000-4-2), not a
// component handling model (HBM/CDM); and matching the rating to the connector-facing PIN, not the part.
const icEsdFloorVolts = 2000

// EsdRatingLimits selects the machine-comparable ESD-rating rows of a spec at or above the credit
// floor: symbol in the alias set, an absolute-max limit (an ESD rating is a max survivable stress),
// a max bound present in V or kV, and the docs/20 comparison gates. Rows failing any gate are skipped.
func EsdRatingLimits(spec *parampb.PartSpec) []*parampb.Parameter {
	var out []*parampb.Parameter
	for _, p := range spec.Parameters {
		// ESD symbols come in many spellings (V_ESD, V(ESD), VESD, ESD_HBM); reduce to alphanumerics.
		sym := alnumUpper(p.Symbol)
		if !esdSymbols[sym] || p.LimitKind != parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX ||
			param.UnderSpecified(p) || !param.MachineComparable(p) {
			continue
		}
		v, ok := esdVolts(p)
		if !ok || v < icEsdFloorVolts {
			continue
		}
		// Only a SYSTEM-level rating (IEC 61000-4-2) protects a connector-exposed signal; a handling
		// model (HBM/CDM) rates the part for assembly, not a field strike (WS3-077 test-model gate).
		if !esdIsSystemLevel(p) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// esdSystemLevelModels are the ESD test-model tokens that rate a part for a SYSTEM-level strike (the
// kind a connector exposes it to): the IEC 61000-4-2 family. A handling model (HBM/CDM/MM) rates the
// part for assembly ONLY, so it must not credit a harness-exposed signal.
var esdSystemLevelModels = map[string]bool{"IEC": true, "CONTACT": true, "AIR": true, "SYSTEM": true}

// esdIsSystemLevel reports whether an ESD-rating row is a system-level (IEC) rating. The test model is a
// DECLARED attribute (esd_test_model = iec | hbm | cdm | ...), never parsed from the Name text — an
// unstated or handling-model rating does not credit (WS3-077): crediting an HBM rating on a harness
// input would hide a real ESD gap, since HBM is not field-strike protection.
func esdIsSystemLevel(p *parampb.Parameter) bool {
	return esdSystemLevelModels[alnumUpper(p.Attributes["esd_test_model"])]
}

// alnumUpper reduces a symbol to its uppercase alphanumeric characters, so the spelling variants a
// datasheet prints an ESD symbol in (V_ESD, V(ESD), VESD) all collapse to one key.
func alnumUpper(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// esdVolts reads an ESD rating's max as volts, accepting the kV unit datasheets often use.
func esdVolts(p *parampb.Parameter) (float64, bool) {
	if p.Value == nil || p.Value.Max == nil {
		return 0, false
	}
	switch p.Unit {
	case "V":
		return p.Value.GetMax(), true
	case "kV":
		return p.Value.GetMax() * 1000, true
	}
	return 0, false
}

// SupplyInputPin reports whether a pin consumes a supply rail, format-neutrally — the entity both
// datasheet rail rules (supply-exceeds-abs-max, rail-nominal-out-of-recommended) quantify over. Since
// WS3-072 PR2 the answer is a plain PinDir == POWER_IN: the ingestion pass (classify.StampPowerInPins)
// fills POWER_IN on a supply-named input pin a reader (EDIF) left under-typed, so every reader now types
// its supply pins the same way KiCad does. The earlier name-role fallback (the WS3-036 interim) is gone.
func SupplyInputPin(m Model, refDes, designator string) bool {
	return m.PinDir(refDes, designator) == ir.PinDirection_PIN_DIRECTION_POWER_IN
}

// SupplyAbsMaxLimits returns the absolute-maximum voltage rows of a PartSpec that name a supply
// pin and are safe to compare: symbol in the supply set, LimitKind ABSOLUTE_MAX, unit V, and fully
// specified (not under-specified, machine-comparable). It is the datasheet lookup behind
// supply-exceeds-abs-max; text-only or under-specified rows are skipped so a rule never compares
// against a value a human must read.
func SupplyAbsMaxLimits(spec *parampb.PartSpec) []*parampb.Parameter {
	var out []*parampb.Parameter
	for _, p := range spec.Parameters {
		sym := strings.ToUpper(strings.ReplaceAll(p.Symbol, " ", ""))
		if !supplySymbols[sym] ||
			p.LimitKind != parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX ||
			p.Unit != "V" || p.Value == nil || p.Value.Max == nil ||
			param.UnderSpecified(p) || !param.MachineComparable(p) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// RecommendedOperatingLimits selects the machine-comparable recommended-operating
// supply-voltage rows of a spec: symbol in the supply alias set, kind
// RECOMMENDED_OPERATING, unit exactly "V", at least one of min/max present, and the
// docs/20 comparison gates (unlike units and text-only conditions are skipped, never
// coerced). Unlike SupplyAbsMaxLimits — a one-sided ceiling that is always conservative
// to apply across every power-in pin — the recommended range is two-sided, so its
// consumer (rail-nominal-out-of-recommended) acts only on a part that declares a SINGLE
// such row: a netlist does not say which power-in pin is which supply, so a multi-supply
// part can't be range-checked without risking a false over/under finding.
func RecommendedOperatingLimits(spec *parampb.PartSpec) []*parampb.Parameter {
	var out []*parampb.Parameter
	for _, p := range spec.Parameters {
		sym := strings.ToUpper(strings.ReplaceAll(p.Symbol, " ", ""))
		if !supplySymbols[sym] ||
			p.LimitKind != parampb.LimitKind_LIMIT_KIND_RECOMMENDED_OPERATING ||
			p.Unit != "V" || p.Value == nil || (p.Value.Min == nil && p.Value.Max == nil) ||
			param.UnderSpecified(p) || !param.MachineComparable(p) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// capRatedVoltageSymbols is the alias set for a capacitor's rated-voltage concept:
// the symbols cap datasheets print it under. Same posture as supplySymbols: the
// vendor spelling lives here in the model layer, never in rule text (docs/20), and
// WS10-004's ontology replaces it.
var capRatedVoltageSymbols = map[string]bool{
	"VDC": true, "WV": true, "VR": true, "VRATED": true,
}

// CapRatedVoltageLimits selects the machine-comparable rated-voltage rows of a cap
// spec: symbol in the alias set (or the printed name saying "rated voltage"), kind
// recommended-operating or absolute-max (a rated voltage is the operating envelope;
// some sheets state it as a maximum), unit exactly "V", a max bound present, and the
// docs/20 comparison gates. Rows failing any gate are skipped, not coerced.
func CapRatedVoltageLimits(spec *parampb.PartSpec) []*parampb.Parameter {
	var out []*parampb.Parameter
	for _, p := range spec.Parameters {
		sym := strings.ToUpper(strings.ReplaceAll(p.Symbol, " ", ""))
		named := strings.Contains(strings.ToLower(p.Name), "rated voltage")
		if (!capRatedVoltageSymbols[sym] && !named) ||
			(p.LimitKind != parampb.LimitKind_LIMIT_KIND_RECOMMENDED_OPERATING &&
				p.LimitKind != parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX) ||
			p.Unit != "V" || p.Value == nil || p.Value.Max == nil ||
			param.UnderSpecified(p) || !param.MachineComparable(p) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// RailMaxVoltage is the "net.max_voltage" fact: the rail voltage a net declares. A
// max_voltage attribute wins when present (the explicit channel; a computed
// worst-case value is WS4); otherwise the name-derived nominal
// (nominalVoltageFromName) is the only evidence a netlist carries. ok is false when
// neither channel yields a number — consumers skip, never guess.
func RailMaxVoltage(n *ir.Net, name string) (volts float64, ok bool) {
	if v := n.Attributes["max_voltage"]; v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return nominalVoltageFromName(name)
}

// voltageToken matches one rail-name token as a nominal voltage: "5V", "+5V", "3V3",
// "12V0", "3.3V". Anchored to the whole token so an embedded digit run ("USB5V",
// "EVENT5") never parses.
var voltageToken = regexp.MustCompile(`^\+?(\d+)(?:V(\d+)?|\.(\d+)V)$`)

// NominalVoltageFromName derives a rail's nominal voltage from its net name — the design-side
// nominal a rail's NAME declares (3V3 -> 3.3), the only voltage evidence a directionless netlist
// carries. ok is false when the name carries no parseable voltage token or two tokens disagree
// ("12V_TO_5V"), because refusing to guess is the contract every voltage rule leans on. Exported so
// an out-of-package rule (the design-intent voltage-domain check) reads the SAME name->volts logic
// the net.nominal_voltage fact projects, rather than re-deriving it and drifting (the C20 left-shift
// rule: interpret a convention once). It is a pure string function, not a Model read, so it sits at
// package scope rather than on the Model.
func NominalVoltageFromName(name string) (volts float64, ok bool) {
	return nominalVoltageFromName(name)
}

// nominalVoltageFromName derives a rail's nominal voltage from its net name — the
// only voltage evidence a netlist carries (the IsPowerRailName precedent). The name
// is split into tokens and each is matched in full; ok is false when no token parses
// or when two tokens disagree ("12V_TO_5V"), because refusing to guess is the
// contract every params rule leans on.
func nominalVoltageFromName(name string) (volts float64, ok bool) {
	found := false
	for _, tok := range strings.FieldsFunc(strings.ToUpper(name), func(r rune) bool {
		return !(r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r == '.' || r == '+')
	}) {
		m := voltageToken.FindStringSubmatch(tok)
		if m == nil {
			continue
		}
		num := m[1]
		if frac := m[2] + m[3]; frac != "" { // only one group can match
			num += "." + frac
		}
		v, err := strconv.ParseFloat(num, 64)
		if err != nil {
			continue
		}
		if found && v != volts {
			return 0, false
		}
		volts, found = v, true
	}
	return volts, found
}
