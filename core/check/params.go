package check

import (
	"regexp"
	"slices"
	"sort"
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
	m.enrichRolesFromParams()
	return m
}

// enrichRolesFromParams adds net roles the DATASHEET establishes, the second evidence tier C9's
// evidence-tier variant admits (agni issue 280). It is the sibling of enrichClassesFromParams and
// runs for the same reason: the mpn map and the seeded specs only exist once a params tier is
// attached, so this cannot happen in the ingestion pass that stamps the name-derived roles.
//
// WHAT MAKES IT WORTH DOING. Until now a net was a rail because of its NAME, and the built-in
// vocabulary is start-anchored (VCC, VDD, +3V3), so a project naming rails function-first had to
// declare its own lexicon or the engine could not see its rails at all. Measured on a real 1700-net
// board that was the difference between 13 rails and 91. A datasheet pin function is evidence no
// name can supply: a terminal the vendor types as a power input is fed by a supply, whatever anyone
// called the net.
//
// ADDITIVE ONLY, which is what makes admitting a tier safe. A datasheet fact can never remove or
// downgrade a role the name or the format established, so a design read WITHOUT --params classifies
// exactly as it did before this existed, and adding a corpus can only ever reveal more rails. It is
// also idempotent: building two models over one design merges rather than duplicating, because the
// merge upgrades a role in place rather than appending.
func (m *irModel) enrichRolesFromParams() {
	fn := m.datasheetPinFunctions()
	if len(fn) == 0 {
		return
	}
	for _, n := range m.d.GetNets() {
		for _, c := range n.GetConnections() {
			role := roleForPinFunction(fn[c.GetComponentRef()+"\x00"+c.GetPinRef()])
			if role == "" {
				continue
			}
			classify.AddNetRole(n, role, ir.RoleSource_ROLE_SOURCE_DATASHEET)
		}
	}
}

// roleForPinFunction maps a vendor pin function onto the net role it evidences. A net reaching a
// power INPUT or a power OUTPUT is a rail either way: one is fed by a supply, the other is driven as
// one. Everything else (signal pins, passives, no-connects) evidences nothing about the net, which is
// why the default is silence rather than a guess.
func roleForPinFunction(f parampb.PinFunction) string {
	switch f {
	case parampb.PinFunction_PIN_FUNCTION_POWER_INPUT, parampb.PinFunction_PIN_FUNCTION_POWER_OUTPUT:
		return NetRoleRail
	case parampb.PinFunction_PIN_FUNCTION_GROUND:
		return NetRoleGround
	default:
		return ""
	}
}

// datasheetPinFunctions resolves every seeded component's design pins onto its spec pins ONCE, keyed
// by "refDes\x00designator". Built up front rather than per net because the alternative is running
// param.ResolvePin per net-connection, which on a board with a few thousand nets means repeating the
// same per-component resolution thousands of times.
//
// A pin that does not resolve to exactly one spec pin contributes nothing. param.ResolvePin refuses
// on an ambiguous name or a name/number disagreement, and a guessed terminal here would stamp a role
// off the wrong pin's function.
func (m *irModel) datasheetPinFunctions() map[string]parampb.PinFunction {
	out := map[string]parampb.PinFunction{}
	for _, c := range m.d.GetComponents() {
		spec := m.PartSpec(c.RefDes)
		if spec == nil || len(spec.GetPins()) == 0 {
			continue
		}
		pkg := ""
		if p := param.PackageForMPN(spec, m.ComponentMPN(c.RefDes)); p != nil {
			pkg = p.GetId()
		}
		for _, pin := range m.Pins() {
			if pin.Component.RefDes != c.RefDes {
				continue
			}
			specPin, err := param.ResolvePin(spec, m.PinName(c.RefDes, pin.Designator), pin.Designator, pkg)
			if err != nil {
				continue
			}
			if f := specPin.GetFunction(); f != parampb.PinFunction_PIN_FUNCTION_UNSPECIFIED {
				out[c.RefDes+"\x00"+pin.Designator] = f
			}
		}
	}
	return out
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
// the breakdown alias set, kind ABSOLUTE_MAX, a unit reducing to volts, a max bound present, and the
// docs/20 comparison gates. Breakdown IS an absolute maximum on a real datasheet (it is the voltage
// past which the part stops being a switch), so unlike OutputVoltageLimits the kind is constrained.
//
// The third member of the connection-aware extractor family: SupplyAbsMaxLimits reads what a part can
// WITHSTAND on its supply, OutputVoltageLimits what a part DELIVERS, and this what a switch can BLOCK.
func FetBreakdownLimits(spec *parampb.PartSpec) []*parampb.Parameter {
	var out []*parampb.Parameter
	for _, p := range spec.Parameters {
		sym := strings.ToUpper(strings.ReplaceAll(p.Symbol, " ", ""))
		if !fetBreakdownSymbols[sym] {
			continue
		}
		q, ok := param.InBaseUnit(p)
		if !ok || q.LimitKind != parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX ||
			q.Unit != "V" || q.Value == nil || q.Value.Max == nil ||
			param.UnderSpecified(q) || !param.MachineComparable(q) {
			continue
		}
		out = append(out, q)
	}
	return out
}

// ocpThresholdSymbols is the alias set for a switch controller's OVERCURRENT-PROTECTION sense
// threshold: the voltage across the external sense resistor at which the controller trips. Same
// posture as supplySymbols, vendor spelling in the model layer and never in rule text.
//
// Narrow on purpose. Every symbol here has to mean "the sense voltage that trips the current limit"
// and nothing else, because the number is divided by a resistance and reported as an ampere rating. A
// symbol that could also mean a clamp level or a comparator reference would produce a confident wrong
// current, which is the failure this rule family costs the most on.
var ocpThresholdSymbols = map[string]bool{
	"VOCP": true, "V(OCP)": true, "VOCTH": true, "V(OC)": true,
	"VILIM": true, "V(ILIM)": true, "VCL": true, "V(CL)": true,
}

// ocpThresholdSymbolNames is the alias set above written as the DISTINCT symbols it recognizes, one
// canonical spelling each, for a rule to declare in Rule.ParamSymbols (WS3-085 sizing).
//
// One spelling per symbol rather than all eight keys, because the two consumers normalize differently.
// OcpThresholdLimits matches on the upper-cased, space-stripped spelling, so V(OCP) and VOCP are two
// keys there; SeedsAnySymbol matches after alnumUpper, which strips the parentheses too, so they are
// ONE key there. Listing both would print the same symbol twice in a review's needs-data note and tell
// an author nothing. TestOcpThresholdSymbolsCoverTheAliasSet holds the two forms to each other so the
// seeding gate and the extractor cannot drift apart.
//
// A SLICE rather than a map, for outputCurrentSymbols' reason: it is read back out into a message, and
// an author reading "no seeded datasheet value for V(OCP)/..." is helped by the ordinary spelling
// coming first.
var ocpThresholdSymbolNames = []string{"V(OCP)", "V(OC)", "VOCTH", "V(ILIM)", "V(CL)"}

// OcpThresholdSymbols returns the overcurrent-threshold alias set for Rule.ParamSymbols, so a review
// runner can tell "this switch trips high enough" from "nothing on this design states a trip
// threshold". Without it a design that seeds no controller resolves no load switch at all, the rule
// reports nothing, and the bound item scores a pass on a check that never ran.
func OcpThresholdSymbols() []string {
	return slices.Clone(ocpThresholdSymbolNames)
}

// OcpThresholdLimits selects the machine-comparable overcurrent-threshold rows of a controller's
// spec: symbol in the alias set, a unit reducing to volts, a max bound present, and the docs/20
// comparison gates. Rows failing any gate are skipped, never coerced.
//
// The limit KIND is deliberately not constrained: a trip threshold is a characteristic or
// recommended-operating row, not an absolute maximum, so filtering to one kind would find nothing on a
// real spec (the same reasoning as OutputVoltageLimits).
//
// Real controller sheets print this row in MILLIVOLTS, and it is the row agni issue 148 was reported
// against: the millivolt spelling used to fail the unit gate, so the resolver found no threshold, no
// load switch resolved, and the item scored a PASS on a check that never ran. param.InBaseUnit
// converts it now, in the one place a scale factor lives, so the returned row is always in volts and
// the rule downstream cannot see the spelling it was seeded in.
func OcpThresholdLimits(spec *parampb.PartSpec) []*parampb.Parameter {
	var out []*parampb.Parameter
	for _, p := range spec.GetParameters() {
		sym := strings.ToUpper(strings.ReplaceAll(p.Symbol, " ", ""))
		if !ocpThresholdSymbols[sym] {
			continue
		}
		q, ok := param.InBaseUnit(p)
		if !ok || q.Unit != "V" || q.Value == nil || q.Value.Max == nil ||
			param.UnderSpecified(q) || !param.MachineComparable(q) {
			continue
		}
		out = append(out, q)
	}
	return out
}

// drainCurrentSymbols is the alias set for a MOSFET's CONTINUOUS drain-current rating. Pulsed drain
// current (IDM, ID(pulse)) is deliberately absent: it is a much larger number under a duty-cycle
// condition, and crediting it against a steady trip current would turn a real over-current into a
// pass.
var drainCurrentSymbols = map[string]bool{
	"ID": true, "IDMAX": true, "ID(MAX)": true, "IDCONT": true, "ID(CONT)": true, "IDC": true,
}

// DrainCurrentLimits selects the machine-comparable continuous drain-current rows of a spec: symbol in
// the alias set, kind ABSOLUTE_MAX, a unit reducing to amps, a max bound present, and the docs/20
// comparison gates. Continuous drain current IS an absolute maximum on a real FET datasheet, so unlike
// OcpThresholdLimits the kind is constrained.
//
// The rating is stated at a case or ambient temperature that the conditions carry, and a real design
// derates well below it. This selects the vendor's number as printed; a rule comparing against it is
// therefore reporting the UNAMBIGUOUS half, a current limit set above the part's own rating, and says
// nothing about whether a limit below the rating is adequately derated.
func DrainCurrentLimits(spec *parampb.PartSpec) []*parampb.Parameter {
	var out []*parampb.Parameter
	for _, p := range spec.GetParameters() {
		sym := strings.ToUpper(strings.ReplaceAll(p.Symbol, " ", ""))
		if !drainCurrentSymbols[sym] {
			continue
		}
		q, ok := param.InBaseUnit(p)
		if !ok || q.LimitKind != parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX ||
			q.Unit != "A" || q.Value == nil || q.Value.Max == nil ||
			param.UnderSpecified(q) || !param.MachineComparable(q) {
			continue
		}
		out = append(out, q)
	}
	return out
}

// onResistanceSymbols is the alias set for a MOSFET's static drain-source ON-resistance.
var onResistanceSymbols = map[string]bool{
	"RDS(ON)": true, "RDSON": true, "RON": true,
}

// OnResistanceLimits selects the machine-comparable RDS(on) rows of a spec: symbol in the alias set, a
// unit reducing to ohms, a max bound present, and the docs/20 comparison gates. The limit kind is not
// constrained, since RDS(on) is a characteristic row.
//
// The ohm SPELLINGS a corpus carries (the hand-encoded "Ohm", the transcribed "Ω", the deprecated
// codepoint) used to live in a local table here, because normalizing two spellings of one unit was
// safe in a way converting two units was not. That distinction stopped needing a separate mechanism
// once param.InBaseUnit existed: it holds both the spellings and the scales, and a milliohm row now
// converts where it used to vanish. RDS(on) is the row where that matters most, since a milliohm is
// the ORDINARY way a sheet prints a modern FET's on-resistance.
//
// A real sheet states RDS(on) SEVERAL TIMES under different gate drives and junction temperatures (the
// seeded BSS138 carries three), so a caller gets several rows and has to say which one it means. There
// is no single "the" on-resistance, and an accessor that picked one silently would be answering a
// question the datasheet does not answer.
func OnResistanceLimits(spec *parampb.PartSpec) []*parampb.Parameter {
	var out []*parampb.Parameter
	for _, p := range spec.GetParameters() {
		sym := strings.ToUpper(strings.ReplaceAll(p.Symbol, " ", ""))
		if !onResistanceSymbols[sym] {
			continue
		}
		q, ok := param.InBaseUnit(p)
		if !ok || q.Unit != param.UnitOhm || q.Value == nil || q.Value.Max == nil ||
			param.UnderSpecified(q) || !param.MachineComparable(q) {
			continue
		}
		out = append(out, q)
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
// output alias set, a unit reducing to volts, a max bound present, and the docs/20 comparison gates.
// The limit KIND is deliberately not constrained: a regulator states its output as a recommended-operating
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
		if !outputSymbols[sym] {
			continue
		}
		q, ok := param.InBaseUnit(p)
		if !ok || q.Unit != "V" || q.Value == nil || q.Value.Max == nil ||
			param.UnderSpecified(q) || !param.MachineComparable(q) {
			continue
		}
		out = append(out, q)
	}
	return out
}

// outputCurrentSymbols is the alias set for a regulator's OUTPUT CURRENT rating: the symbols
// datasheets print it under, matched after alnumUpper normalization so I_OUT, I OUT and IOUT(MAX) all
// reduce to a key here. Same posture as supplySymbols, the vendor spelling lives in the model layer
// and never in rule text (docs/20).
//
// It is the current-axis counterpart of outputSymbols and is deliberately narrow in the same way:
// these are ratings for what the part DELIVERS, so a symbol that could mean an input current (IIN,
// IQ, ISD) must not be in here or a sizing rule would compare a rail's demand against the wrong
// number. ILIM is likewise excluded: a current LIMIT is where the part folds back, not the current it
// is rated to deliver continuously, and crediting it as capacity would over-rate every part that
// states both.
// It is a SLICE rather than a map, unlike its neighbours here, because these symbols are also read
// back out (OutputCurrentSymbols) into a review's needs-data message, and an author reading "no seeded
// datasheet value for IOUT/IO/..." is helped by the ordinary spelling coming first. Map iteration
// order would not give that, and sorting would lead with ICONT.
var outputCurrentSymbols = []string{"IOUT", "IO", "IOUTMAX", "IOMAX", "ICONT", "ICONTMAX", "ILOAD", "IOUTDC"}

// OutputCurrentSymbols returns the output-current alias set, for a rule to declare in
// Rule.ParamSymbols so the review runner can tell "the design is within budget" from "nothing on this
// design states an output current" (WS3-095). It is the same list OutputCurrentLimits matches on, so
// the seeding gate and the extractor cannot drift apart.
func OutputCurrentSymbols() []string {
	return slices.Clone(outputCurrentSymbols)
}

// OutputCurrentLimits selects the machine-comparable OUTPUT-CURRENT rows of a spec: symbol in the
// output-current alias set, a unit reducing to amps, a max bound present, and the docs/20 comparison
// gates.
//
// The limit KIND is deliberately not constrained, for OutputVoltageLimits' reason: a regulator states
// its output current as a recommended-operating or characteristic row rather than an absolute maximum,
// so filtering to one kind would find nothing on a real spec.
//
// The other row agni issue 148 was reported against. A regulator stating IOUT in MILLIAMPS used to
// fail the unit gate, so the rail-budget rules found no supply for the rail, compared nothing, and the
// item scored a PASS while the rail was genuinely over-subscribed. Milliamps are the ordinary spelling
// for a sub-amp regulator, so a spec transcribed as printed hit this without doing anything unusual.
func OutputCurrentLimits(spec *parampb.PartSpec) []*parampb.Parameter {
	var out []*parampb.Parameter
	for _, p := range spec.Parameters {
		if !slices.Contains(outputCurrentSymbols, alnumUpper(p.Symbol)) {
			continue
		}
		q, ok := param.InBaseUnit(p)
		if !ok || q.Unit != "A" || q.Value == nil || q.Value.Max == nil ||
			param.UnderSpecified(q) || !param.MachineComparable(q) {
			continue
		}
		out = append(out, q)
	}
	return out
}

// SeedsAnySymbol reports whether ANY component on the design carries a seeded datasheet parameter for
// one of syms, comparing after the same alnumUpper normalization the alias sets use so a spec written
// I_OUT answers for a declared IOUT. It is the review runner's WS3-097 gate generalized to a rule that
// declares its symbols (Rule.ParamSymbols) rather than an inline query that names exactly one: false
// means the rule had nothing to join against, so zero findings is a data gap and not a clean design.
//
// Normalization lives here rather than at the call site for the reason the alias sets do: symbol
// spelling is a model-layer concern, so a caller passes the symbols it wants and never the rules for
// matching them.
func SeedsAnySymbol(m Model, syms []string) bool {
	if len(syms) == 0 {
		return false
	}
	want := make(map[string]bool, len(syms))
	for _, s := range syms {
		want[alnumUpper(s)] = true
	}
	for _, c := range m.Components() {
		spec := m.PartSpec(c.RefDes)
		if spec == nil {
			continue
		}
		for _, p := range spec.GetParameters() {
			if want[alnumUpper(p.GetSymbol())] {
				return true
			}
		}
	}
	return false
}

// UnmetDependency names one datasheet fact a check reached for and did not find: which part, which
// symbol, and whether the part has any seeded spec at all.
//
// The unit is the PART, not the placement. A PartSpec is per-MPN, so ten instances of one component
// are one thing to go seed; deduplicating by MPN is what makes this a work list rather than a
// restatement of the bill of materials.
type UnmetDependency struct {
	MPN          string
	Manufacturer string // as the seeded spec states it; empty when SpecAbsent
	Symbol       string
	// SpecAbsent distinguishes "no spec for this part" from "a spec that states no such symbol",
	// because the next step differs: extract a document, versus find one fact in one already read.
	SpecAbsent bool
}

// UnseededSymbols reports the (part, symbol) pairs that SeedsAnySymbol found nothing for. It is the
// same walk that decides the needs-data outcome, keeping what that one discards: SeedsAnySymbol
// answers "does ANY part seed this", which is the right question for the verdict and the wrong one
// for acting on it.
//
// classes gates the walk to an item's applies_to_class when it declares one, for the same reason the
// computed-n/a path does: without it a symbol needed by three parts would name every part on the
// design that lacks it, and a work list nobody can act on is not better than prose.
//
// The result is deduplicated by (mpn, symbol) and sorted, so a run written and re-read reproduces
// byte for byte. A component with no resolvable MPN yields nothing: there is no part to name, and a
// dependency naming nothing cannot be acted on.
func UnseededSymbols(m Model, syms []string, classes []string) []UnmetDependency {
	if len(syms) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var out []UnmetDependency
	for _, c := range m.Components() {
		if len(classes) > 0 && !hasAnyClass(m, c.RefDes, classes) {
			continue
		}
		mpn := m.ComponentMPN(c.RefDes)
		if mpn == "" {
			continue
		}
		spec := m.PartSpec(c.RefDes)
		have := map[string]bool{}
		for _, prm := range spec.GetParameters() {
			have[alnumUpper(prm.GetSymbol())] = true
		}
		for _, s := range syms {
			if s == "" || have[alnumUpper(s)] {
				continue
			}
			key := strings.ToUpper(mpn) + "\x00" + alnumUpper(s)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, UnmetDependency{
				MPN: mpn, Manufacturer: spec.GetManufacturer(), Symbol: s, SpecAbsent: spec == nil,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MPN != out[j].MPN {
			return out[i].MPN < out[j].MPN
		}
		return out[i].Symbol < out[j].Symbol
	})
	return out
}

func hasAnyClass(m Model, refDes string, classes []string) bool {
	for _, cl := range classes {
		if m.HasClass(refDes, ComponentClass(cl)) {
			return true
		}
	}
	return false
}

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
// floor: symbol in the alias set, an absolute-max limit (an ESD rating is a max survivable stress), a
// max bound present in a unit reducing to volts, and the docs/20 comparison gates. Rows failing any
// gate are skipped.
//
// This extractor already converted, alone in this file, because an ESD rating is printed in KILOVOLTS
// as often as in volts and a rule that skipped the kV spelling would have credited almost nothing. It
// carried its own two-case scale to do it. That special case is now the general one, so the local
// converter is gone and this reads like its nine neighbours.
func EsdRatingLimits(spec *parampb.PartSpec) []*parampb.Parameter {
	var out []*parampb.Parameter
	for _, p := range spec.Parameters {
		// ESD symbols come in many spellings (V_ESD, V(ESD), VESD, ESD_HBM); reduce to alphanumerics.
		sym := alnumUpper(p.Symbol)
		if !esdSymbols[sym] {
			continue
		}
		q, ok := param.InBaseUnit(p)
		if !ok || q.Unit != "V" || q.LimitKind != parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX ||
			q.Value == nil || q.Value.Max == nil ||
			param.UnderSpecified(q) || !param.MachineComparable(q) {
			continue
		}
		if q.Value.GetMax() < icEsdFloorVolts {
			continue
		}
		// Only a SYSTEM-level rating (IEC 61000-4-2) protects a connector-exposed signal; a handling
		// model (HBM/CDM) rates the part for assembly, not a field strike (WS3-077 test-model gate).
		if !esdIsSystemLevel(q) {
			continue
		}
		out = append(out, q)
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

// SupplyInputPin reports whether a pin consumes a supply rail, format-neutrally — the entity both
// datasheet rail rules (supply-exceeds-abs-max, rail-nominal-out-of-recommended) quantify over. Since
// WS3-072 PR2 the answer is a plain PinDir == POWER_IN: the ingestion pass (classify.StampPowerInPins)
// fills POWER_IN on a supply-named input pin a reader (EDIF) left under-typed, so every reader now types
// its supply pins the same way KiCad does. The earlier name-role fallback (the WS3-036 interim) is gone.
func SupplyInputPin(m Model, refDes, designator string) bool {
	return m.PinDir(refDes, designator) == ir.PinDirection_PIN_DIRECTION_POWER_IN
}

// SupplyAbsMaxLimits returns the absolute-maximum voltage rows of a PartSpec that name a supply
// pin and are safe to compare: symbol in the supply set, LimitKind ABSOLUTE_MAX, a unit reducing to
// volts, and fully specified (not under-specified, machine-comparable). It is the datasheet lookup
// behind supply-exceeds-abs-max; text-only or under-specified rows are skipped so a rule never
// compares against a value a human must read.
func SupplyAbsMaxLimits(spec *parampb.PartSpec) []*parampb.Parameter {
	var out []*parampb.Parameter
	for _, p := range spec.Parameters {
		sym := strings.ToUpper(strings.ReplaceAll(p.Symbol, " ", ""))
		if !supplySymbols[sym] {
			continue
		}
		q, ok := param.InBaseUnit(p)
		if !ok || q.LimitKind != parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX ||
			q.Unit != "V" || q.Value == nil || q.Value.Max == nil ||
			param.UnderSpecified(q) || !param.MachineComparable(q) {
			continue
		}
		out = append(out, q)
	}
	return out
}

// RecommendedOperatingLimits selects the machine-comparable recommended-operating
// supply-voltage rows of a spec: symbol in the supply alias set, kind
// RECOMMENDED_OPERATING, a unit reducing to volts, at least one of min/max present, and
// the docs/20 comparison gates (unrecognized units and text-only conditions are skipped,
// never coerced). Unlike SupplyAbsMaxLimits — a one-sided ceiling that is always conservative
// to apply across every power-in pin — the recommended range is two-sided, so its
// consumer (rail-nominal-out-of-recommended) acts only on a part that declares a SINGLE
// such row: a netlist does not say which power-in pin is which supply, so a multi-supply
// part can't be range-checked without risking a false over/under finding.
func RecommendedOperatingLimits(spec *parampb.PartSpec) []*parampb.Parameter {
	var out []*parampb.Parameter
	for _, p := range spec.Parameters {
		sym := strings.ToUpper(strings.ReplaceAll(p.Symbol, " ", ""))
		if !supplySymbols[sym] {
			continue
		}
		q, ok := param.InBaseUnit(p)
		if !ok || q.LimitKind != parampb.LimitKind_LIMIT_KIND_RECOMMENDED_OPERATING ||
			q.Unit != "V" || q.Value == nil || (q.Value.Min == nil && q.Value.Max == nil) ||
			param.UnderSpecified(q) || !param.MachineComparable(q) {
			continue
		}
		out = append(out, q)
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
// some sheets state it as a maximum), a unit reducing to volts, a max bound present, and
// the docs/20 comparison gates. Rows failing any gate are skipped, not coerced.
func CapRatedVoltageLimits(spec *parampb.PartSpec) []*parampb.Parameter {
	var out []*parampb.Parameter
	for _, p := range spec.Parameters {
		sym := strings.ToUpper(strings.ReplaceAll(p.Symbol, " ", ""))
		named := strings.Contains(strings.ToLower(p.Name), "rated voltage")
		if !capRatedVoltageSymbols[sym] && !named {
			continue
		}
		q, ok := param.InBaseUnit(p)
		if !ok ||
			(q.LimitKind != parampb.LimitKind_LIMIT_KIND_RECOMMENDED_OPERATING &&
				q.LimitKind != parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX) ||
			q.Unit != "V" || q.Value == nil || q.Value.Max == nil ||
			param.UnderSpecified(q) || !param.MachineComparable(q) {
			continue
		}
		out = append(out, q)
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
