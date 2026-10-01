package relations

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/core/facts"
	"github.com/panyam/agni/core/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/internal/netgraph"
)

// The design fact base (WS3-004) is a set of named, typed, provenanced relations over the IR and
// its datasheet joins. Rules and ad-hoc queries read the same vocabulary. Facts derives the tuples and
// nothing here evaluates a query (that is core/query).
//
// The relations are a DERIVED PROJECTION of the Model, regenerated on demand and never a second
// authoritative schema (C8). Every fact carries provenance (an IR site or a datasheet page) so an
// answer built from it stays checkable. Names are neutral IR/param concepts, not format specifics
// (C9). Each relation's full contract is its page under facts/docs/.
const (
	RelNetMaxVoltage  = "net.max_voltage" // net.max_voltage(net, volts): a net's declared rail voltage. doc: facts/docs/net.max_voltage.md
	RelComponentMPN   = "component.mpn"   // component.mpn(ref_des, mpn): the design-side part identity. doc: facts/docs/component.mpn.md
	RelParam          = "param.max"       // param.max(mpn, symbol, value, conditions): a datasheet parameter. doc: facts/docs/param.max.md
	RelPartAudience   = "part.audience"   // part.audience(mpn, who): a team/license entitled to see a part's datasheet data. doc: facts/docs/part.audience.md
	RelComponentOnNet = "component.net"   // component.net(ref_des, net): a component sits on a net. doc: facts/docs/component.net.md

	// net.nominal_voltage is the nominal a RAIL's name declares (3V3 -> 3.3). Distinct from
	// net.max_voltage, which prefers an explicit max_voltage attribute over the name (WS3-082).
	RelNetNominalVoltage = "net.nominal_voltage" // net.nominal_voltage(net, volts): name-derived rail nominal. doc: facts/docs/net.nominal_voltage.md

	// net.signal_level is the same name-derived number on a net that is NOT a rail, split from
	// net.nominal_voltage so a rule says which set it means (issue 194).
	RelNetSignalLevel = "net.signal_level" // net.signal_level(net, volts): name-derived level on a non-rail net. doc: facts/docs/net.signal_level.md

	// param.range adds the lower bound and the limit kind that param.max(mpn, symbol, max) lacks, so
	// absolute-max and recommended-operating rows on one symbol stay apart (WS3-082).
	RelParamRange = "param.range" // param.range(mpn, symbol, kind, min, max): a two-sided datasheet limit. doc: facts/docs/param.range.md

	// param.typ is its own relation rather than a column on param.range because A TYP IS NOT A
	// BOUND. Comparing a rail against one as though it were a limit gives a confident wrong answer
	// (agni issue 545).
	RelParamTyp = "param.typ" // param.typ(mpn, symbol, typ): a parameter's typical value. doc: facts/docs/param.typ.md

	// param.prov is where a datasheet value was read from, so a datalog rule can carry the citation
	// onto its findings (WS10-012). doc is the resolved SourceDoc title, not the doc_ref id; method
	// and confidence come from check.DatasheetProvFor. THE PAGE BINDS AS A STRING, since a unitless
	// number would unify with a voltage (agni issue 545).
	RelParamProv = "param.prov" // param.prov(mpn, symbol, doc, page, section): a datasheet value's Citation. doc: facts/docs/param.prov.md

	// param.unit is the unit a parameter is PRINTED in, kept queryable because param and param.range
	// publish SI base units (agni issue 165). String-valued, so no ordering comparison binds it.
	RelParamUnit = "param.unit" // param.unit(mpn, symbol, unit): the unit a parameter is printed in. doc: facts/docs/param.unit.md

	// The PIN tier of the datasheet relations (agni issue 189) states limits per TERMINAL rather
	// than per (mpn, symbol). Keyed by the spec-local Pin.id, since a printed name is not unique. Mapping a
	// DESIGN pin onto an id is param.ResolvePin, not a datalog join, because resolution can refuse.
	RelParamPin      = "param.pin"       // param.pin(mpn, pin, name, function): a declared pin of a part. doc: facts/docs/param.pin.md
	RelParamPinRange = "param.pin_range" // param.pin_range(mpn, pin, symbol, kind, min, max): a limit bound to one pin. doc: facts/docs/param.pin_range.md

	// param.pin_relation bounds the DIFFERENCE between two pins of one part. Subject and reference
	// are ordered, since the bound is on subject minus reference.
	RelParamPinRelation = "param.pin_relation" // param.pin_relation(mpn, subject_pin, reference_pin, modality, min, max): a pin-to-pin bound. doc: facts/docs/param.pin_relation.md

	// Board-tier relations (WS1-006) are derived per-net values in millimetres, not raw geometry.
	RelBoardTrackWidth = "board.track_width" // board.track_width(net, mm): the net's MINIMUM copper width. doc: facts/docs/board.track_width.md
	RelBoardViaDrill   = "board.via_drill"   // board.via_drill(net, mm): the net's MINIMUM via drill. doc: facts/docs/board.via_drill.md
	RelBoardLayer      = "board.layer"       // board.layer(net, layer): a layer the net's copper occupies. doc: facts/docs/board.layer.md

	// Pin-level and per-net netlist relations (WS3-038), each a projection of an existing Model
	// method. pin.net is absent for an unconnected pin, so `not pin.net(?r,?p,?_)` reads as
	// "unconnected".
	RelPin               = "component.pin"          // component.pin(ref_des, pin): a part-type pin of a placed component. doc: facts/docs/component.pin.md
	RelPinRole           = "pin.role"               // pin.role(ref_des, pin, role): derived power/ground/anode/cathode. doc: facts/docs/pin.role.md
	RelPinType           = "pin.type"               // pin.type(ref_des, pin, etype): electrical type (power_in, input, ...). doc: facts/docs/pin.type.md
	RelPinNet            = "pin.net"                // pin.net(ref_des, pin, net): the net a pin is on (absent if none). doc: facts/docs/pin.net.md
	RelPinName           = "pin.name"               // pin.name(ref_des, pin, name): the part type's functional name for the pin (absent if unnamed). doc: facts/docs/pin.name.md
	RelNetPinCount       = "net.pin_count"          // net.pin_count(net, count): connections on a net. doc: facts/docs/net.pin_count.md
	RelComponentNetCount = "component.net_count"    // component.net_count(ref_des, count): distinct nets a component touches. doc: facts/docs/component.net_count.md
	RelHasNCChannel      = "design.has_nc_channel"  // design.has_nc_channel(present): one row when the design can express no-connect. doc: facts/docs/design.has_nc_channel.md
	RelTypesPowerOut     = "design.types_power_out" // design.types_power_out(present): one row when the source format types power-output pins (WS3-072). doc: facts/docs/design.types_power_out.md
	RelRail              = "net.rail"               // net.rail(net): the net is a power/ground rail (Model.IsPowerRail). doc: facts/docs/net.rail.md
	RelFeedback          = "net.feedback"           // net.feedback(net): the net is a regulator feedback/sense node (naming lexicon). doc: facts/docs/net.feedback.md
	RelSwitching         = "net.switching"          // net.switching(net): the net is a regulator power-stage node (naming lexicon). doc: facts/docs/net.switching.md
	RelNetRole           = "net.role"               // net.role(net, role): a role the net carries, one row per role. doc: facts/docs/net.role.md
	RelNetAttr           = "net.attr"               // net.attr(net, key, value): a net-level attribute. doc: facts/docs/net.attr.md
	RelComponentAttr     = "component.attr"         // component.attr(ref_des, key, value): a component-level attribute. doc: facts/docs/component.attr.md

	// Device-class and net-attribute relations (WS3-074). component.class has one row per tag in the
	// device_classes SET (WS3-071), so a family tag answers too. net.ground isolates the ground half
	// of net.rail, which covers both.
	RelComponentClass = "component.class" // component.class(ref_des, class): a device class the part is in. doc: facts/docs/component.class.md
	RelNetGround      = "net.ground"      // net.ground(net): the net is a ground rail (name-derived). doc: facts/docs/net.ground.md
	RelNetExternal    = "net.external"    // net.external(net): the net may extend onto an unread sheet. doc: facts/docs/net.external.md

	// component.esd_rated is the credit the esd-protection Go rule gives, from the same
	// EsdRatingLimits extractor, keyed by ref_des (WS3-076). Empty without --params.
	RelEsdRated = "component.esd_rated" // component.esd_rated(ref_des): part carries a floor-clearing ESD rating. doc: facts/docs/component.esd_rated.md

	// component.device_class is the class the part's DATASHEET declares (PartSpec.device_class),
	// joined by MPN (WS10-013). Empty without --params. WithParamProvider also merges it into
	// component.class's set, so HasClass answers from it too.
	RelComponentDeviceClass = "component.device_class" // component.device_class(ref_des, class): the datasheet-declared device class. doc: facts/docs/component.device_class.md

	// bus(label, kind) is a reader-detected bus construct not yet expanded (WS1-034 Phase 1). label
	// is empty for an anonymous wire.
	RelBus = "bus" // doc: facts/docs/bus.md

	// entity(name, kind) is the ENUMERATION relation, the one a search starts from, since every other
	// relation misses whatever it does not reach (a part with no connections). kind uses the
	// check.Kind* vocabulary a finding subject carries. Pins are absent because their identity is two
	// fields; component.pin(ref_des, pin) enumerates them.
	RelEntity = "entity" // entity(name, kind): a component, net or bus exists under this name. doc: facts/docs/entity.md
	// RelUnresolvedSymbol is keyed by ref_des, NOT by the symbol reference, so a query can ask what
	// KIND of parts a missing library cost (WS1-052).
	RelUnresolvedSymbol = "reader.unresolved_symbol" // doc: facts/docs/reader.unresolved_symbol.md

	// Reader-diagnostic relations (WS3-081). A diagnostic earns a query relation only when it carries
	// an entity key to join on; point-geometry ones (dangling endpoints) stay rule-scoped.
	RelRefDesCollision = "reader.ref_des_collision" // reader.ref_des_collision(ref_des): a designator shared by >1 part. doc: facts/docs/reader.ref_des_collision.md
	RelPinNetConflict  = "reader.pin_net_conflict"  // reader.pin_net_conflict(ref_des, pin, net): the read put a pin on >1 net. doc: facts/docs/reader.pin_net_conflict.md

	// net.bus_like is a shared-distribution net, the predicate the series-reach walk stops at
	// (WS3-080). Not bus(label, kind), which is an unmodeled bus LABEL.
	RelNetBusLike = "net.bus_like" // doc: facts/docs/net.bus_like.md

	// net.connector_signal is the SCOPE the ESD rules share (WS3-061). It reads net attributes and
	// the no-connect channel that no relation exposes, so datalog cannot rebuild it, and a dropped
	// guard is a false FAIL on a rail or an unconnected pad.
	RelExternalSignalNet = "net.connector_signal" // net.connector_signal(net): connector-facing signal net, the ESD scope. doc: facts/docs/net.connector_signal.md

	// Derived net properties (WS3-088) say what the DESIGN does, for comparing against an intent
	// declaration.
	RelNetBias      = "net.bias"       // net.bias(net, level): a bias resistor holds the net high or low. doc: facts/docs/net.bias.md
	RelNetACCoupled = "net.ac_coupled" // net.ac_coupled(net): a SERIES capacitor carries the net. doc: facts/docs/net.ac_coupled.md

	// Net-class relations (WS3-105) carry the TOOL-assigned class string. NOT named net.class, which
	// would read as the derived role space and invite a join that matches nothing.
	// design.has_netclass separates "no net in this class" from "this design has no classes" (only
	// KiCad supplies them).
	RelNetNetClass = "net.netclass"        // net.netclass(net, class): the tool-assigned net class. doc: facts/docs/net.netclass.md
	RelHasNetClass = "design.has_netclass" // design.has_netclass(present): one row when the design assigns net classes at all. doc: facts/docs/design.has_netclass.md

	// Net-class DEFINITIONS (WS3-111), keyed by CLASS, in millimetres like the board tier. These are
	// the raw per-class rows; the cascaded per-NET values are below.
	RelNetClassClearance   = "netclass.clearance"       // netclass.clearance(class, mm). doc: facts/docs/netclass.clearance.md
	RelNetClassTrackWidth  = "netclass.track_width"     // netclass.track_width(class, mm). doc: facts/docs/netclass.track_width.md
	RelNetClassViaDiameter = "netclass.via_diameter"    // netclass.via_diameter(class, mm). doc: facts/docs/netclass.via_diameter.md
	RelNetClassViaDrill    = "netclass.via_drill"       // netclass.via_drill(class, mm). doc: facts/docs/netclass.via_drill.md
	RelHasNetClassDefs     = "design.has_netclass_defs" // design.has_netclass_defs(present). doc: facts/docs/design.has_netclass_defs.md

	// The CASCADED per-net values. A declared-vs-actual rule joins THESE, never the per-class rows,
	// because a net in two classes matches two of those. Only the two quantities with a board-tier
	// counterpart are derived.
	RelNetDeclaredTrackWidth = "net.declared_track_width" // net.declared_track_width(net, mm). doc: facts/docs/net.declared_track_width.md
	RelNetDeclaredViaDrill   = "net.declared_via_drill"   // net.declared_via_drill(net, mm). doc: facts/docs/net.declared_via_drill.md
)

// unitVolt and unitMillimetre are the BASE units the numeric relations publish. One dimension has
// ONE spelling across every relation.
//
// MILLIMETRES ARE NOT THE SI BASE for length, on purpose, since every board format and board query
// uses mm (`?w < 0.2`). A datasheet length would have to be projected as mm to join.
//
// Counts carry NO base unit. An empty base unit is polymorphic, so `?c < 5` works; a new count
// must be listed in dimensionlessNumericRelations (facts_test.go).
const (
	unitVolt       = "V"
	unitMillimetre = "mm"
)

// Facts projects the Model into the fact base, sorted so two calls on one Model are equal. A
// relation is empty when the Model lacks its tier (no --params means no param rows); Facts never
// fabricates.
func Facts(m check.Model) []facts.Row {
	var out []facts.Row
	out = append(out, netMaxVoltageFacts(m)...)
	out = append(out, netNominalVoltageFacts(m)...)
	out = append(out, netSignalLevelFacts(m)...)
	out = append(out, componentMPNFacts(m)...)
	out = append(out, paramFacts(m)...)
	out = append(out, paramRangeFacts(m)...)
	out = append(out, paramTypFacts(m)...)
	out = append(out, paramUnitFacts(m)...)
	out = append(out, paramProvFacts(m)...)
	out = append(out, paramPinFacts(m)...)
	out = append(out, paramPinRangeFacts(m)...)
	out = append(out, paramPinRelationFacts(m)...)
	out = append(out, audienceFacts(m)...)
	out = append(out, entityFacts(m)...)
	out = append(out, componentOnNetFacts(m)...)
	out = append(out, pinFacts(m)...)
	out = append(out, netPinCountFacts(m)...)
	out = append(out, componentNetCountFacts(m)...)
	out = append(out, ncChannelFacts(m)...)
	out = append(out, typesPowerOutFacts(m)...)
	out = append(out, railFacts(m)...)
	out = append(out, feedbackFacts(m)...)
	out = append(out, switchingFacts(m)...)
	out = append(out, netRoleFacts(m)...)
	out = append(out, netAttrFacts(m)...)
	out = append(out, componentAttrFacts(m)...)
	out = append(out, componentClassFacts(m)...)
	out = append(out, esdRatedFacts(m)...)
	out = append(out, componentDeviceClassFacts(m)...)
	out = append(out, netGroundFacts(m)...)
	out = append(out, netExternalFacts(m)...)
	out = append(out, busFacts(m)...)
	out = append(out, unresolvedSymbolFacts(m)...)
	out = append(out, refDesCollisionFacts(m)...)
	out = append(out, pinNetConflictFacts(m)...)
	out = append(out, netBusLikeFacts(m)...)
	out = append(out, externalSignalNetFacts(m)...)
	out = append(out, netBiasFacts(m)...)
	out = append(out, netACCoupledFacts(m)...)
	out = append(out, netNetClassFacts(m)...)
	out = append(out, hasNetClassFacts(m)...)
	out = append(out, netClassDefFacts(m)...)
	out = append(out, hasNetClassDefsFacts(m)...)
	out = append(out, netDeclaredFacts(m)...)
	out = append(out, boardFacts(m)...)
	sortFacts(out)
	return out
}

// sortFacts orders fact rows by (relation, subject, object), for both Facts and SpecLibFacts.
func sortFacts(out []facts.Row) {
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Relation != out[j].Relation {
			return out[i].Relation < out[j].Relation
		}
		if out[i].Subject != out[j].Subject {
			return out[i].Subject < out[j].Subject
		}
		return out[i].Object < out[j].Object
	})
}

func netMaxVoltageFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, n := range m.Nets() {
		if v, ok := check.RailMaxVoltage(n, n.Name); ok {
			vv := v
			out = append(out, facts.Row{Relation: RelNetMaxVoltage, Subject: n.Name, Value: fmt.Sprintf("%gV", v), Num: &vv, BaseUnit: unitVolt, Cites: cite(irCite(n.Prov))})
		}
	}
	return out
}

// netNominalVoltageFacts emits the name-derived nominal of each RAIL (3V3 -> 3.3). It reads only
// the NAME, never the max_voltage attribute, and a name with no parseable nominal yields no row.
//
// THE RAIL GATE IS THE POINT (agni issue 194). NominalVoltageFromName token-scans the whole name,
// so without the gate a signal net like `U3_12_U7_4_3V3` projected 3.3 as a rail nominal. That
// level goes to net.signal_level instead.
//
// The gate is on the RELATION, not on check.NominalVoltageFromName, which is a pure string
// function. A Go rule holding a net must gate for itself (Model.IsRailNet), as rule_pin_tracking
// does.
func netNominalVoltageFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, n := range m.Nets() {
		if !m.IsRailNet(n) || m.IsRegulatorInternalNet(n) {
			continue
		}
		if v, ok := check.NominalVoltageFromName(n.Name); ok {
			vv := v
			out = append(out, facts.Row{Relation: RelNetNominalVoltage, Subject: n.Name, Value: fmt.Sprintf("%gV", v), Num: &vv, BaseUnit: unitVolt, Cites: cite(irCite(n.Prov))})
		}
	}
	return out
}

// netSignalLevelFacts emits the voltage a NON-RAIL net's name carries, the other half of the
// issue-194 split.
//
// The two relations are DISJOINT but not exhaustive (agni 679). A regulator internal such as
// "12V_FB" gets NO row in either, because the number in its name is a DIFFERENT net's voltage (the
// tap sits near the internal reference, typically 0.6V to 0.8V). Moving those nets to this relation
// would state the same wrong number under a new name. Ground is a rail role, so a ground net stays
// on the nominal side.
func netSignalLevelFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, n := range m.Nets() {
		if m.IsRailNet(n) || m.IsRegulatorInternalNet(n) {
			continue
		}
		if v, ok := check.NominalVoltageFromName(n.Name); ok {
			vv := v
			out = append(out, facts.Row{Relation: RelNetSignalLevel, Subject: n.Name, Value: fmt.Sprintf("%gV", v), Num: &vv, BaseUnit: unitVolt, Cites: cite(irCite(n.Prov))})
		}
	}
	return out
}

func componentMPNFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, c := range m.Components() {
		if mpn := m.ComponentMPN(c.RefDes); mpn != "" {
			out = append(out, facts.Row{Relation: RelComponentMPN, Subject: c.RefDes, Value: mpn, Cites: cite(irCite(c.Prov))})
		}
	}
	return out
}

// paramFacts emits every parameter of each JOINED datasheet spec, not only the ones a rule
// consumes, keyed by mpn and deduped by it.
func paramFacts(m check.Model) []facts.Row {
	var out []facts.Row
	seen := map[string]bool{}
	for _, c := range m.Components() {
		mpn := m.ComponentMPN(c.RefDes)
		if mpn == "" || seen[mpn] {
			continue
		}
		spec := m.PartSpec(c.RefDes)
		if spec == nil {
			continue
		}
		seen[mpn] = true
		out = append(out, specParamRows(mpn, spec)...)
	}
	return out
}

// EVERY NUMBER A PARAMETER RELATION EMITS IS IN ITS SI BASE UNIT, reduced through
// param.InBaseUnit (agni issue 165, C24). A row has no unit slot, so a spec seeded 4600 mV would
// otherwise compare as 4600 against a 5.0 V threshold.
//
// A row whose unit has no known scale keeps its symbol, kind, conditions and citation with its
// NUMERIC slots empty rather than being dropped. That is safe only because the evaluator refuses to
// ORDER an absent number against a present one. See facts/docs/param.unit.md.

// specParamRows projects the `param.max` rows of one PartSpec, the upper bound in its SI base unit.
// Shared by paramFacts and SpecLibFacts so the two emit identical rows.
func specParamRows(mpn string, spec *parampb.PartSpec) []facts.Row {
	out := make([]facts.Row, 0, len(spec.Parameters))
	for _, p := range spec.Parameters {
		q, ok := param.InBaseUnit(p)
		if !ok {
			// The unit has no known scale, so there is no number to publish. The row still appears,
			// carrying its symbol, conditions and citation, with the numeric slot EMPTY: an
			// unmeasurable value must not be orderable, and evalCompare refuses to order an absent
			// number against a present one.
			out = append(out, facts.Row{Relation: RelParam, Subject: mpn, Object: p.GetSymbol(), Conditions: conditionsText(p.GetConditions()), Cites: cite(check.Citation(spec, p))})
			continue
		}
		f := facts.Row{Relation: RelParam, Subject: mpn, Object: q.Symbol, Value: rangeText(q.Value), BaseUnit: q.Unit, Conditions: conditionsText(q.Conditions), Cites: cite(check.Citation(spec, p))}
		if q.Value != nil && q.Value.Max != nil {
			v := *q.Value.Max
			f.Num = &v
		}
		out = append(out, f)
	}
	return out
}

// specParamPinRows projects the `param.pin` rows of one PartSpec: the spec-local id (Object), the
// printed name (Value) and the function (Qualifier). The ID IS THE JOIN KEY, since a part often
// prints one name on several terminals. Empty for a spec seeded before pin binding.
func specParamPinRows(mpn string, spec *parampb.PartSpec) []facts.Row {
	out := make([]facts.Row, 0, len(spec.GetPins()))
	for _, pin := range spec.GetPins() {
		out = append(out, facts.Row{
			Relation: RelParamPin, Subject: mpn, Object: pin.GetId(),
			Value: pin.GetName(), Qualifier: param.PinFunctionToken(pin.GetFunction()),
			Cites: cite(check.PinCitation(spec, pin)),
		})
	}
	return out
}

// specParamPinRangeRows projects the `param.pin_range` rows of one PartSpec: one per (parameter,
// bound pin), carrying the symbol (Value), the limit kind (Qualifier) and both bounds in SI base
// units (Min/Num). A parameter bound to four pins emits FOUR rows.
//
// PART-WIDE ROWS ARE ABSENT. A parameter with no pin_refs is about the die, and emitting it per pin
// would read as each terminal carrying that limit; those rows live on `param.range`. An
// unconvertible unit keeps the row with both numeric slots empty, as specParamRangeRows does.
func specParamPinRangeRows(mpn string, spec *parampb.PartSpec) []facts.Row {
	var out []facts.Row
	for _, p := range spec.GetParameters() {
		// Redundant with the empty loop below; it states the exclusion and skips a unit conversion.
		if len(p.GetPinRefs()) == 0 {
			continue
		}
		q, ok := param.InBaseUnit(p)
		for _, ref := range p.GetPinRefs() {
			f := facts.Row{
				Relation: RelParamPinRange, Subject: mpn, Object: ref,
				Value: p.GetSymbol(), Qualifier: param.LimitKindToken(p.GetLimitKind()),
				Conditions: conditionsText(p.GetConditions()), Cites: cite(check.Citation(spec, p)),
			}
			if ok {
				f.Value, f.Qualifier = q.Symbol, param.LimitKindToken(q.LimitKind)
				f.BaseUnit, f.Conditions = q.Unit, conditionsText(q.Conditions)
				if q.Value != nil {
					// BOTH bounds reduce together, as in specParamRangeRows.
					if q.Value.Min != nil {
						v := *q.Value.Min
						f.Min = &v
					}
					if q.Value.Max != nil {
						v := *q.Value.Max
						f.Num = &v
					}
				}
			}
			out = append(out, f)
		}
	}
	return out
}

// paramPinFacts emits the declared pins of each joined part, deduped by MPN and empty without
// --params.
func paramPinFacts(m check.Model) []facts.Row {
	return perJoinedSpec(m, specParamPinRows)
}

// paramPinRangeFacts emits the pin-bound limits of each joined part, deduped by MPN and empty
// without --params.
func paramPinRangeFacts(m check.Model) []facts.Row {
	return perJoinedSpec(m, specParamPinRangeRows)
}

// specParamPinRelationRows projects the `param.pin_relation` rows of one PartSpec: the two pin ids
// in SUBTRACTION ORDER (Object is the subject, Value the reference), the modality (Qualifier) and
// the bound on their difference in SI base units (Min/Num). Swapping the ids inverts the
// requirement.
//
// TRACKING ONLY, since PinRelationKind has one member. A second kind must revisit this projection,
// which spends no column on the kind. An unconvertible unit keeps the row with both numeric slots
// empty.
func specParamPinRelationRows(mpn string, spec *parampb.PartSpec) []facts.Row {
	var out []facts.Row
	for _, r := range spec.GetRelations() {
		if r.GetKind() != parampb.PinRelationKind_PIN_RELATION_KIND_TRACKING {
			continue
		}
		f := facts.Row{
			Relation: RelParamPinRelation, Subject: mpn,
			Object: r.GetSubjectPinRef(), Value: r.GetReferencePinRef(),
			Qualifier:  param.ModalityToken(r.GetModality()),
			Conditions: conditionsText(r.GetConditions()),
			Cites:      cite(check.RelationCitation(spec, r)),
		}
		if base, exp, ok := param.BaseUnit(r.GetUnit()); ok {
			scale := math.Pow(10, float64(exp))
			f.BaseUnit = base
			// BOTH bounds reduce together, as in specParamRangeRows.
			if d := r.GetDifference(); d != nil {
				if d.Min != nil {
					v := d.GetMin() * scale
					f.Min = &v
				}
				if d.Max != nil {
					v := d.GetMax() * scale
					f.Num = &v
				}
			}
		}
		out = append(out, f)
	}
	return out
}

// paramPinRelationFacts emits the pin-to-pin constraints of each joined part, deduped by MPN and
// empty without --params.
func paramPinRelationFacts(m check.Model) []facts.Row {
	return perJoinedSpec(m, specParamPinRelationRows)
}

// perJoinedSpec runs one per-spec projector once per MPN over the joined specs. A PartSpec
// describes the TYPE, so emitting per component would multiply every row by its placement count.
func perJoinedSpec(m check.Model, rows func(string, *parampb.PartSpec) []facts.Row) []facts.Row {
	var out []facts.Row
	seen := map[string]bool{}
	for _, c := range m.Components() {
		mpn := m.ComponentMPN(c.RefDes)
		if mpn == "" || seen[mpn] {
			continue
		}
		spec := m.PartSpec(c.RefDes)
		if spec == nil {
			continue
		}
		seen[mpn] = true
		out = append(out, rows(mpn, spec)...)
	}
	return out
}

// specParamUnitRows projects the `param.unit` rows of one PartSpec: the unit each parameter is
// PRINTED in. EVERY parameter is emitted, including one whose unit has no known scale.
func specParamUnitRows(mpn string, spec *parampb.PartSpec) []facts.Row {
	out := make([]facts.Row, 0, len(spec.Parameters))
	for _, p := range spec.Parameters {
		out = append(out, facts.Row{
			Relation: RelParamUnit, Subject: mpn, Object: p.GetSymbol(), Value: p.GetUnit(),
			Cites: cite(check.Citation(spec, p)),
		})
	}
	return out
}

// paramUnitFacts emits the printed unit of each joined datasheet parameter, deduped by MPN and empty
// without --params.
func paramUnitFacts(m check.Model) []facts.Row {
	var out []facts.Row
	seen := map[string]bool{}
	for _, c := range m.Components() {
		mpn := m.ComponentMPN(c.RefDes)
		if mpn == "" || seen[mpn] {
			continue
		}
		spec := m.PartSpec(c.RefDes)
		if spec == nil {
			continue
		}
		seen[mpn] = true
		out = append(out, specParamUnitRows(mpn, spec)...)
	}
	return out
}

// specParamRangeRows projects the `param.range` rows of one PartSpec: the kind token (Value), the
// lower bound (Min) and the upper bound (Num). Shared by paramRangeFacts and SpecLibFacts.
func specParamRangeRows(mpn string, spec *parampb.PartSpec) []facts.Row {
	out := make([]facts.Row, 0, len(spec.Parameters))
	for _, p := range spec.Parameters {
		q, ok := param.InBaseUnit(p)
		if !ok {
			// As in specParamRows, the row stays with both numeric slots empty.
			out = append(out, facts.Row{Relation: RelParamRange, Subject: mpn, Object: p.GetSymbol(), Value: param.LimitKindToken(p.GetLimitKind()), Conditions: conditionsText(p.GetConditions()), Cites: cite(check.Citation(spec, p))})
			continue
		}
		f := facts.Row{Relation: RelParamRange, Subject: mpn, Object: q.Symbol, Value: param.LimitKindToken(q.LimitKind), BaseUnit: q.Unit, Conditions: conditionsText(q.Conditions), Cites: cite(check.Citation(spec, p))}
		if q.Value != nil {
			// BOTH bounds are reduced. Converting only the max would leave a "3000..3.6" row that
			// fires the opposite finding.
			if q.Value.Min != nil {
				v := *q.Value.Min
				f.Min = &v
			}
			if q.Value.Max != nil {
				v := *q.Value.Max
				f.Num = &v
			}
		}
		out = append(out, f)
	}
	return out
}

// specParamTypRows projects the `param.typ` rows of one PartSpec, one per parameter that states a
// typ. An absent typ stays absent rather than arriving as a zero. An unconvertible unit keeps the
// row with no number.
func specParamTypRows(mpn string, spec *parampb.PartSpec) []facts.Row {
	out := make([]facts.Row, 0, len(spec.Parameters))
	for _, p := range spec.Parameters {
		if p.GetValue() == nil || p.GetValue().Typ == nil {
			continue
		}
		f := facts.Row{Relation: RelParamTyp, Subject: mpn, Object: p.GetSymbol(), Conditions: conditionsText(p.GetConditions()), Cites: cite(check.Citation(spec, p))}
		q, ok := param.InBaseUnit(p)
		if !ok {
			out = append(out, f)
			continue
		}
		f.Object, f.BaseUnit, f.Conditions = q.Symbol, q.Unit, conditionsText(q.Conditions)
		v := *q.Value.Typ
		f.Num = &v
		out = append(out, f)
	}
	return out
}

// paramTypFacts emits the typical value of each joined datasheet parameter, deduped by MPN and empty
// without --params.
func paramTypFacts(m check.Model) []facts.Row {
	return perJoinedSpec(m, specParamTypRows)
}

// paramRangeFacts emits `param.range` over the same joined specs paramFacts reads, deduped by MPN
// and empty without --params.
func paramRangeFacts(m check.Model) []facts.Row {
	var out []facts.Row
	seen := map[string]bool{}
	for _, c := range m.Components() {
		mpn := m.ComponentMPN(c.RefDes)
		if mpn == "" || seen[mpn] {
			continue
		}
		spec := m.PartSpec(c.RefDes)
		if spec == nil {
			continue
		}
		seen[mpn] = true
		out = append(out, specParamRangeRows(mpn, spec)...)
	}
	return out
}

// specParamProvRows projects the `param.prov` rows of one PartSpec: the resolved SourceDoc title
// (Value), the page as a string (Qualifier, agni issue 545) and the table or figure (Conditions).
// Shared by paramProvFacts and SpecLibFacts.
func specParamProvRows(mpn string, spec *parampb.PartSpec) []facts.Row {
	out := make([]facts.Row, 0, len(spec.Parameters))
	for _, p := range spec.Parameters {
		out = append(out, facts.Row{
			Relation: RelParamProv,
			Subject:  mpn,
			Object:   p.Symbol,
			Value:    check.DocTitle(spec, p.GetProv().GetDocRef()),
			// The page is a locator, so it binds as a string (agni issue 545).
			Qualifier:  strconv.Itoa(int(p.GetProv().GetPage())),
			Conditions: p.GetProv().GetTableOrFigure(),
			Cites:      cite(check.Citation(spec, p)),
		})
	}
	return out
}

// paramProvFacts emits where each joined datasheet parameter was read from, deduped by MPN and
// empty without --params.
func paramProvFacts(m check.Model) []facts.Row {
	var out []facts.Row
	seen := map[string]bool{}
	for _, c := range m.Components() {
		mpn := m.ComponentMPN(c.RefDes)
		if mpn == "" || seen[mpn] {
			continue
		}
		spec := m.PartSpec(c.RefDes)
		if spec == nil {
			continue
		}
		seen[mpn] = true
		out = append(out, specParamProvRows(mpn, spec)...)
	}
	return out
}

// audienceFacts projects `part.audience` over the joined specs, one row per entitled team or
// license (param.Audience, WS10-010). Record-only; nothing enforces it (WS10-011).
func audienceFacts(m check.Model) []facts.Row {
	var out []facts.Row
	seen := map[string]bool{}
	for _, c := range m.Components() {
		mpn := m.ComponentMPN(c.RefDes)
		if mpn == "" || seen[mpn] {
			continue
		}
		spec := m.PartSpec(c.RefDes)
		if spec == nil {
			continue
		}
		seen[mpn] = true
		out = append(out, audienceRows(mpn, spec)...)
	}
	return out
}

// audienceRows projects one part's `part.audience` rows, shared by Facts and SpecLibFacts. A part
// with no audience annotation emits nothing.
func audienceRows(mpn string, spec *parampb.PartSpec) []facts.Row {
	var out []facts.Row
	for _, who := range param.Audience(spec) {
		out = append(out, facts.Row{Relation: RelPartAudience, Subject: mpn, Object: who})
	}
	return out
}

// SpecLibFacts projects the per-spec datasheet relations of a whole seeded corpus with NO design
// join (WS10-010), which is what `agni query --speclib` searches. The three pin relations are not
// included. Rows are sorted as Facts sorts them.
func SpecLibFacts(specs []*parampb.PartSpec) []facts.Row {
	var out []facts.Row
	for _, spec := range specs {
		if spec.GetMpn() == "" {
			continue
		}
		out = append(out, specParamRows(spec.GetMpn(), spec)...)
		out = append(out, specParamRangeRows(spec.GetMpn(), spec)...)
		out = append(out, specParamTypRows(spec.GetMpn(), spec)...)
		out = append(out, specParamUnitRows(spec.GetMpn(), spec)...)
		out = append(out, specParamProvRows(spec.GetMpn(), spec)...)
		out = append(out, audienceRows(spec.GetMpn(), spec)...)
	}
	sortFacts(out)
	return out
}

// entityFacts emits one row per named component, net and unmodeled bus, citing its own IR site. An
// unnamed net or anonymous bus wire emits nothing, since an empty name answers no search.
func entityFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, c := range m.Components() {
		if c.RefDes == "" {
			continue
		}
		out = append(out, facts.Row{Relation: RelEntity, Subject: c.RefDes, Value: check.KindComponent, Cites: cite(irCite(c.Prov))})
	}
	for _, n := range m.Nets() {
		if n.Name == "" {
			continue
		}
		out = append(out, facts.Row{Relation: RelEntity, Subject: n.Name, Value: check.KindNet, Cites: cite(irCite(n.Prov))})
	}
	for _, b := range m.UnmodeledBuses() {
		if b.GetLabel() == "" {
			continue
		}
		out = append(out, facts.Row{Relation: RelEntity, Subject: b.GetLabel(), Value: check.KindBus, Cites: cite(irCite(b.GetProv()))})
	}
	return out
}

func componentOnNetFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, n := range m.Nets() {
		for _, conn := range n.Connections {
			out = append(out, facts.Row{Relation: RelComponentOnNet, Subject: conn.ComponentRef, Object: n.Name, Cites: cite(irCite(n.Prov))})
		}
	}
	return out
}

// pinFacts projects every part-type pin of every placed component: pin, pin.role, pin.type, pin.net
// and pin.name. check.RoleUnknown emits no pin.role row, and an unconnected pin no pin.net row.
// Empty on a bare netlist with no part-pin data.
func pinFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, p := range m.Pins() {
		ref, des := p.Component.RefDes, p.Designator
		// One slice shared by the pin's rows, which is safe because rows are read-only downstream.
		cites := cite(irCite(p.Component.Prov))
		out = append(out, facts.Row{Relation: RelPin, Subject: ref, Object: des, Cites: cites})
		if role := m.PinRole(ref, des); role != check.RoleUnknown {
			out = append(out, facts.Row{Relation: RelPinRole, Subject: ref, Object: des, Value: string(role), Cites: cites})
		}
		out = append(out, facts.Row{Relation: RelPinType, Subject: ref, Object: des, Value: check.DirString(m.PinDir(ref, des)), Cites: cites})
		if net := m.PinNetName(ref, des); net != "" {
			out = append(out, facts.Row{Relation: RelPinNet, Subject: ref, Object: des, Value: net, Cites: cites})
		}
		// The FUNCTIONAL name, beside the designator the other rows key on (agni issue 517).
		// Emitted only when declared. KiCad's "~" means "no name" and reaches the IR verbatim, so
		// it reads as absent, as in classifyPinRole and resolveEndpoint.
		if name := m.PinName(ref, des); name != "" && name != "~" {
			out = append(out, facts.Row{Relation: RelPinName, Subject: ref, Object: des, Value: name, Cites: cites})
		}
	}
	return out
}

// netPinCountFacts emits each net's connection count, which tells a single-pin stub from a real
// net.
func netPinCountFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, n := range m.Nets() {
		c := float64(len(n.Connections))
		out = append(out, facts.Row{Relation: RelNetPinCount, Subject: n.Name, Num: &c, Cites: cite(irCite(n.Prov))})
	}
	return out
}

// componentNetCountFacts emits how many DISTINCT nets each component touches. It reads connections,
// not part-type pins, so it answers on a bare netlist and agrees with component.net. Every
// component gets a row (0 when unwired), and a ref seen only in a connection gets one citing
// nothing (agni issue 727).
func componentNetCountFacts(m check.Model) []facts.Row {
	nets := map[string]map[string]bool{}
	var order []string
	touch := func(ref string) {
		if nets[ref] == nil {
			nets[ref] = map[string]bool{}
			order = append(order, ref)
		}
	}
	prov := map[string]*ir.Provenance{}
	for _, c := range m.Components() {
		touch(c.RefDes)
		prov[c.RefDes] = c.Prov
	}
	for _, n := range m.Nets() {
		for _, conn := range n.Connections {
			touch(conn.ComponentRef)
			nets[conn.ComponentRef][n.Name] = true
		}
	}
	out := make([]facts.Row, 0, len(order))
	for _, ref := range order {
		c := float64(len(nets[ref]))
		out = append(out, facts.Row{Relation: RelComponentNetCount, Subject: ref, Num: &c, Cites: cite(irCite(prov[ref]))})
	}
	return out
}

// ncChannelFacts emits one row when the design can express intentional no-connect (a
// NO_CONNECT-typed pin or an nc-marker net), and none otherwise, so a `design.has_nc_channel(?_)` gate
// fails closed.
func ncChannelFacts(m check.Model) []facts.Row {
	if m.HasNoConnectChannel() {
		return []facts.Row{{Relation: RelHasNCChannel, Subject: "true", Cites: cite("design")}}
	}
	return nil
}

// typesPowerOutFacts emits one row when the source format types power-OUTPUT pins (KiCad and gEDA
// do, EDIF and IPC do not; see Model.FormatTypesPowerOut). It is the queryable twin of the spec
// fact power-input-not-driven gates on.
func typesPowerOutFacts(m check.Model) []facts.Row {
	if m.FormatTypesPowerOut() {
		return []facts.Row{{Relation: RelTypesPowerOut, Subject: "true", Cites: cite("design")}}
	}
	return nil
}

// railFacts emits one row per net Model.IsPowerRail accepts (power-driven, global, ground or the
// rail role), so a profile's pull-up check can write `net.reaches(?sig, ?r), net.rail(?r)`.
func railFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, n := range m.Nets() {
		if m.IsPowerRail(n.Name) {
			out = append(out, facts.Row{Relation: RelRail, Subject: n.Name, Cites: cite(irCite(n.Prov))})
		}
	}
	return out
}

// feedbackFacts emits one row per net the naming lexicon reads as a regulator feedback or sense
// node (WS3-069/067), so a rule can write `net.rail(?n), not net.feedback(?n)`.
func feedbackFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, n := range m.Nets() {
		if check.NetHasRole(n, ir.Role_ROLE_FEEDBACK, m.IsFeedbackName) {
			out = append(out, facts.Row{Relation: RelFeedback, Subject: n.Name, Cites: cite(irCite(n.Prov))})
		}
	}
	return out
}

// switchingFacts emits one row per net the naming lexicon reads as a regulator power-stage node
// (switch node, bootstrap node, or a vendor spelling of either; agni 680). With net.feedback it is the
// pair a coverage rule subtracts from net.rail.
func switchingFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, n := range m.Nets() {
		if check.NetHasRole(n, ir.Role_ROLE_SWITCHING, m.IsSwitchingName) {
			out = append(out, facts.Row{Relation: RelSwitching, Subject: n.Name, Cites: cite(irCite(n.Prov))})
		}
	}
	return out
}

// netRoleFacts emits one row per ROLE a net carries, the net-side twin of component.class (agni
// 691). It iterates classify.AllNetRoles rather than the stamped set, so an unstamped net still
// answers through the name fallback.
//
// net.rail(?n) is NOT net.role(?n, "rail"). net.rail is Model.IsPowerRail, a CONCLUSION over several
// signals, while net.role is what was STAMPED. net.feedback and net.switching are exact shorthands.
func netRoleFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, n := range m.Nets() {
		for _, role := range classify.AllNetRoles() {
			if m.HasAnyRole(n, role) {
				out = append(out, facts.Row{Relation: RelNetRole, Subject: n.Name, Value: classify.RoleToken(role), Cites: cite(irCite(n.Prov))})
			}
		}
	}
	return out
}

// netAttrFacts emits each net-level attribute as net.attr(net, key, value), the twin of
// component.attr. Attributes are DECLARED by the source file and roles DERIVED by the engine, so
// net.attr and net.role stay separate relations.
func netAttrFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, n := range m.Nets() {
		for k, v := range n.GetAttributes() {
			out = append(out, facts.Row{Relation: RelNetAttr, Subject: n.Name, Object: k, Value: v, Cites: cite(irCite(n.Prov))})
		}
	}
	return out
}

// componentAttrFacts emits each component-level attribute as component.attr(ref, key, value). An
// interface profile binds its host this way, e.g. component.attr(?ref, "interface", "SPI_NOR").
func componentAttrFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, c := range m.Components() {
		for k, v := range c.Attributes {
			out = append(out, facts.Row{Relation: RelComponentAttr, Subject: c.RefDes, Object: k, Value: v, Cites: cite(irCite(c.Prov))})
		}
	}
	return out
}

// componentClassFacts emits component.class(ref, class) once per tag in the device_classes set
// (WS3-071), so a TVS answers both "tvs" and "diode". The class is the canonical lowercase name.
// An unclassified component emits nothing.
func componentClassFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, c := range m.Components() {
		for _, cl := range m.Classes(c.RefDes) {
			out = append(out, facts.Row{Relation: RelComponentClass, Subject: c.RefDes, Value: string(cl), Cites: cite(irCite(c.Prov))})
		}
	}
	return out
}

// esdRatedFacts emits component.esd_rated(ref) for each component whose joined spec carries an ESD
// rating at or above the credit floor (check.EsdRatingLimits). It cites the datasheet ESD rows, not
// the component's IR site. Empty without --params.
func esdRatedFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, c := range m.Components() {
		spec := m.PartSpec(c.RefDes)
		if spec == nil {
			continue
		}
		limits := check.EsdRatingLimits(spec)
		if len(limits) == 0 {
			continue
		}
		// EVERY qualifying rating, not limits[0], since IEC 61000-4-2 air and contact discharge
		// are printed separately (agni issue 546).
		cites := make([]string, 0, len(limits))
		for _, l := range limits {
			if s := check.Citation(spec, l); s != "" {
				cites = append(cites, s)
			}
		}
		out = append(out, facts.Row{Relation: RelEsdRated, Subject: c.RefDes, Cites: cites})
	}
	return out
}

// componentDeviceClassFacts emits component.device_class(ref, class) for each component whose
// joined spec declares a device_class (WS10-013), citing the spec's source document. Empty without
// --params.
//
// The value goes through classify.NormalizeDeviceClass (WS3-044), the same fold
// classify.StampClassesFromSpecs applies, so this relation and component.class agree on the key.
func componentDeviceClassFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, c := range m.Components() {
		spec := m.PartSpec(c.RefDes)
		if spec == nil || spec.GetDeviceClass() == "" {
			continue
		}
		cl := string(classify.NormalizeDeviceClass(spec.GetDeviceClass()))
		out = append(out, facts.Row{Relation: RelComponentDeviceClass, Subject: c.RefDes, Value: cl, Cites: cite(specDocCite(spec))})
	}
	return out
}

// specDocCite cites the first source document's title for a PartSpec-level fact with no
// per-parameter provenance. A missing title renders "unknown source", as check.Citation does.
func specDocCite(spec *parampb.PartSpec) string {
	doc := "unknown source"
	if docs := spec.GetDocs(); len(docs) > 0 && docs[0].GetTitle() != "" {
		doc = docs[0].GetTitle()
	}
	return fmt.Sprintf("datasheet %q", doc)
}

// netGroundFacts emits net.ground(net) for each ground net (Model.IsGroundNet). net.rail covers both
// power and ground, so a supply rail is `net.rail(?r), not net.ground(?r)`.
func netGroundFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, n := range m.Nets() {
		if m.IsGroundNet(n) {
			out = append(out, facts.Row{Relation: RelNetGround, Subject: n.Name, Cites: cite(irCite(n.Prov))})
		}
	}
	return out
}

// netExternalFacts emits net.external(net) for each net the read flagged as possibly extending onto
// an unread sheet (netgraph.AttrExternal), so a datalog rule can skip it as the Go rules do.
func netExternalFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, n := range m.Nets() {
		if n.GetAttributes()[netgraph.AttrExternal] == "true" {
			out = append(out, facts.Row{Relation: RelNetExternal, Subject: n.Name, Cites: cite(irCite(n.Prov))})
		}
	}
	return out
}

// busFacts emits bus(label, kind) for each reader-detected unmodeled bus (WS1-034 Phase 1). label
// is empty for an anonymous bus wire.
func busFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, b := range m.UnmodeledBuses() {
		out = append(out, facts.Row{Relation: RelBus, Subject: b.GetLabel(), Value: b.GetKind(), Cites: cite(irCite(b.GetProv()))})
	}
	return out
}

// unresolvedSymbolFacts emits reader.unresolved_symbol(ref_des, symref) once per PLACEMENT that lost its
// pins (WS1-052), keyed by ref_des so it joins the netlist relations.
func unresolvedSymbolFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, u := range m.UnresolvedSymbols() {
		for _, ref := range u.GetRefDes() {
			out = append(out, facts.Row{Relation: RelUnresolvedSymbol, Subject: ref, Value: u.GetSymref(), Cites: cite(irCite(u.GetProv()))})
		}
	}
	return out
}

// refDesCollisionFacts emits reader.ref_des_collision(ref) for each designator used by more than
// one part (WS3-081). EVERY colliding instance is cited, since where the duplicates are is the
// finding (agni issue 546).
func refDesCollisionFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, c := range m.RefDesCollisions() {
		cites := make([]string, 0, len(c.Instances))
		for _, inst := range c.Instances {
			if s := irCite(inst); s != "" {
				cites = append(cites, s)
			}
		}
		out = append(out, facts.Row{Relation: RelRefDesCollision, Subject: c.GetRefDes(), Cites: cites})
	}
	return out
}

// pinNetConflictFacts emits reader.pin_net_conflict(ref, pin, net) once PER net when the read put
// one pin on more than one net (WS3-081).
func pinNetConflictFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, pc := range m.PinNetConflicts() {
		for _, net := range pc.Nets {
			out = append(out, facts.Row{Relation: RelPinNetConflict, Subject: pc.RefDes, Object: pc.Pin, Value: net, Cites: cite(irCite(pc.Prov))})
		}
	}
	return out
}

// netBusLikeFacts emits net.bus_like(net) for each net check.IsBusLike accepts, the predicate the
// series-reach walk stops at (WS3-080).
func netBusLikeFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, n := range m.Nets() {
		if check.IsBusLike(m, n) {
			out = append(out, facts.Row{Relation: RelNetBusLike, Subject: n.Name, Cites: cite(irCite(n.Prov))})
		}
	}
	return out
}

// externalSignalNetFacts emits net.connector_signal(net) for each net check.ExternalSignalNet
// accepts, the scope the ESD rules share. Empty on a design with no connectors.
func externalSignalNetFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, n := range m.Nets() {
		if check.ExternalSignalNet(m, n) {
			out = append(out, facts.Row{Relation: RelExternalSignalNet, Subject: n.Name, Cites: cite(irCite(n.Prov))})
		}
	}
	return out
}

// netBiasFacts emits net.bias(net, "high"|"low") for each net a bias resistor holds at a rail. A net
// with no bias yields no row, so `not net.bias(?n,?_)` reads as "unbiased".
func netBiasFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, n := range m.Nets() {
		up, down := check.NetBias(m, n)
		level := ""
		switch {
		case up:
			level = "high"
		case down:
			level = "low"
		default:
			continue
		}
		out = append(out, facts.Row{Relation: RelNetBias, Subject: n.Name, Value: level, Cites: cite(irCite(n.Prov))})
	}
	return out
}

// netACCoupledFacts emits net.ac_coupled(net) for each net a SERIES capacitor carries. A decoupling
// cap (far side on ground or a rail) does not count.
func netACCoupledFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, n := range m.Nets() {
		if check.ACCoupled(m, n) {
			out = append(out, facts.Row{Relation: RelNetACCoupled, Subject: n.Name, Cites: cite(irCite(n.Prov))})
		}
	}
	return out
}

// netNetClassFacts emits net.netclass(net, class), the tool's class string verbatim, ONE ROW PER
// (net, class) PAIR (WS1-050). A net in the implicit default class yields no row. Empty for every
// source but a KiCad project; see hasNetClassFacts.
func netNetClassFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, n := range m.Nets() {
		for _, c := range n.NetClasses {
			out = append(out, facts.Row{Relation: RelNetNetClass, Subject: n.Name, Value: c, Cites: cite(irCite(n.Prov))})
		}
	}
	return out
}

// hasNetClassFacts emits one design.has_netclass row when the design assigns net classes at all, the
// queryable twin of check.CapNetClass. Without it "no net is in class HV" and "no classes" read the
// same.
func hasNetClassFacts(m check.Model) []facts.Row {
	if m.HasNetClasses() {
		return []facts.Row{{Relation: RelHasNetClass, Subject: "true", Cites: cite("design")}}
	}
	return nil
}

// boardFacts projects the board tier (WS1-006), per net the MINIMUM track width and via drill in
// mm and the layers its copper occupies. Empty without a board tier. The cite names the board net,
// since a derived BoardNet carries no file span.
func boardFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, bn := range m.BoardNets() {
		cites := cite("board net " + bn.Net)
		if w, ok := minSegmentWidthNm(bn.Segments); ok {
			mm := nmToMM(w)
			out = append(out, facts.Row{Relation: RelBoardTrackWidth, Subject: bn.Net, Value: mmStr(mm), Num: &mm, BaseUnit: unitMillimetre, Cites: cites})
		}
		if d, ok := minViaDrillNm(bn.Vias); ok {
			mm := nmToMM(d)
			out = append(out, facts.Row{Relation: RelBoardViaDrill, Subject: bn.Net, Value: mmStr(mm), Num: &mm, BaseUnit: unitMillimetre, Cites: cites})
		}
		for _, layer := range netLayers(bn.Segments) {
			out = append(out, facts.Row{Relation: RelBoardLayer, Subject: bn.Net, Object: layer, Cites: cites})
		}
	}
	return out
}

func minSegmentWidthNm(segs []check.BoardSeg) (int64, bool) {
	found := false
	var min int64
	for _, s := range segs {
		if !found || s.Width < min {
			min, found = s.Width, true
		}
	}
	return min, found
}

func minViaDrillNm(vias []check.BoardVia) (int64, bool) {
	found := false
	var min int64
	for _, v := range vias {
		if !found || v.Drill < min {
			min, found = v.Drill, true
		}
	}
	return min, found
}

func netLayers(segs []check.BoardSeg) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range segs {
		if s.Layer != "" && !seen[s.Layer] {
			seen[s.Layer] = true
			out = append(out, s.Layer)
		}
	}
	sort.Strings(out)
	return out
}

// netClassDefParams is the ir.Constraint param key for each declared scalar, paired with the
// relation that projects it.
var netClassDefParams = []struct {
	param string
	rel   string
}{
	{"clearance", RelNetClassClearance},
	{"track_width", RelNetClassTrackWidth},
	{"via_diameter", RelNetClassViaDiameter},
	{"via_drill", RelNetClassViaDrill},
}

// netClassDefFacts emits the RAW per-class declarations, one row per (class, quantity) the project
// stated. An unstated field yields no row, since it cascades to a lower-priority class rather than
// being zero.
func netClassDefFacts(m check.Model) []facts.Row {
	var out []facts.Row
	for _, c := range m.NetClassDefs() {
		for _, p := range netClassDefParams {
			mm, ok := parseMM(c.GetParams()[p.param])
			if !ok {
				continue
			}
			v := mm
			out = append(out, facts.Row{Relation: p.rel, Subject: c.GetName(), Value: mmStr(v), Num: &v, BaseUnit: unitMillimetre, Cites: cite("net_settings")})
		}
	}
	return out
}

// hasNetClassDefsFacts is the design-level marker for class DEFINITIONS, independent of
// design.has_netclass because a project can assign nets to classes it never defines. A
// declared-vs-actual rule gates on it so "no definitions" does not read as clean.
func hasNetClassDefsFacts(m check.Model) []facts.Row {
	if len(m.NetClassDefs()) == 0 {
		return nil
	}
	return []facts.Row{{Relation: RelHasNetClassDefs, Subject: "true", Cites: cite("design")}}
}

// netDeclaredFacts emits each net's EFFECTIVE declared value per quantity, resolved through
// check.NetClassCascade.
//
// KiCad composes the effective netclass PER FIELD, not per class. Classes sort by priority (Default
// last) and each field comes from the first class stating it. There is no single winning class. A quantity
// no class states yields no row.
func netDeclaredFacts(m check.Model) []facts.Row {
	defs := m.NetClassDefs()
	if len(defs) == 0 {
		return nil
	}
	cascade := check.NewNetClassCascade(defs)
	var out []facts.Row
	for _, n := range m.Nets() {
		for _, q := range []struct {
			param string
			rel   string
		}{
			{"track_width", RelNetDeclaredTrackWidth},
			{"via_drill", RelNetDeclaredViaDrill},
		} {
			v, cls, ok := cascade.Declared(n.GetNetClasses(), q.param)
			if !ok {
				continue
			}
			out = append(out, facts.Row{
				Relation: q.rel, Subject: n.GetName(), Value: mmStr(v), Num: &v, BaseUnit: unitMillimetre,
				Cites: cite("net_settings:" + cls),
			})
		}
	}
	return out
}

// parseMM reads a declared millimetre scalar. Absent and unparseable both read as "not stated", so
// an unreadable value never becomes a limit.
func parseMM(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// nmToMM converts a board dimension from nanometres to millimetres, the unit the board relations
// publish.
func nmToMM(nm int64) float64 { return float64(nm) / 1e6 }

func mmStr(mm float64) string { return fmt.Sprintf("%gmm", mm) }

// cite wraps one rendered citation as the slice facts.Row carries. A fact with SEVERAL sources
// builds the slice itself (refDesCollisionFacts, esdRatedFacts). An empty string yields no citation,
// so a failed resolution "cites nothing", which TestEveryFactCitesSomething catches.
func cite(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

// irCite renders an IR provenance as a one-line citation, the source file narrowed by the reader's
// native id when present. A nil provenance renders "".
func irCite(p *ir.Provenance) string {
	if p == nil {
		return ""
	}
	src := p.SourceFile
	if src == "" {
		src = "(unknown source)"
	}
	if p.NativeId != "" {
		src += ":" + p.NativeId
	}
	return src
}

// rangeText renders a parameter's range as min..max / <=max / >=min / =typ.
func rangeText(v *parampb.RangeValue) string {
	if v == nil {
		return ""
	}
	switch {
	case v.Min != nil && v.Max != nil:
		return fmt.Sprintf("%g..%g", *v.Min, *v.Max)
	case v.Max != nil:
		return fmt.Sprintf("<=%g", *v.Max)
	case v.Min != nil:
		return fmt.Sprintf(">=%g", *v.Min)
	case v.Typ != nil:
		return fmt.Sprintf("=%g", *v.Typ)
	}
	return ""
}

// conditionsText renders a parameter's test conditions, preferring the source's own raw text
// and falling back to "symbol op value unit" when raw is absent.
func conditionsText(cs []*parampb.Condition) string {
	if len(cs) == 0 {
		return ""
	}
	var parts []string
	for _, c := range cs {
		switch {
		case c.Raw != "":
			parts = append(parts, c.Raw)
		case c.Eq != nil:
			parts = append(parts, fmt.Sprintf("%s=%g%s", c.Symbol, *c.Eq, c.Unit))
		case c.Min != nil && c.Max != nil:
			parts = append(parts, fmt.Sprintf("%s=%g..%g%s", c.Symbol, *c.Min, *c.Max, c.Unit))
		default:
			parts = append(parts, c.Symbol)
		}
	}
	return strings.Join(parts, "; ")
}
