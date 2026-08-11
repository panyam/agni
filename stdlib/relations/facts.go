package relations

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/internal/netgraph"
)

// The design fact base (WS3-004): the reads a rule declares, captured as named, typed,
// provenanced relations over the IR and its datasheet joins. A rule asserts a property over
// these relations; an engineer's ad-hoc search is an arbitrary query over the same ones — so
// rules and search unify on one vocabulary. This file is the fact-capture DISCIPLINE only, not a
// query engine (that is WS3-029): Facts derives the tuples; nothing here evaluates a query.
//
// The relations are a DERIVED PROJECTION of the Model, regenerated on demand — never a second
// authoritative schema (CONSTRAINTS C8). Every fact carries provenance so an answer built from
// it stays checkable (an IR site or a datasheet page), which is the verifiability the whole
// datasheet story leans on. Relation names are neutral IR/param concepts (C9), not format
// specifics, so the fact base is as neutral as the IR.
//
// The seed schema is the four relations the cap-voltage rule reads; more accrue as each rule
// adopts the discipline (its declared Reads name the relations it consumes).
const (
	RelNetMaxVoltage  = "net.max_voltage"  // net.max_voltage(net, volts): a net's declared rail voltage. doc: facts/docs/net.max_voltage.md
	RelComponentMPN   = "component.mpn"    // component.mpn(ref_des, mpn): the design-side part identity. doc: facts/docs/component.mpn.md
	RelParam          = "param"            // param(mpn, symbol, value, conditions): a datasheet parameter. doc: facts/docs/param.md
	RelPartAudience   = "part.audience"    // part.audience(mpn, who): a team/license entitled to see a part's datasheet data. doc: facts/docs/part.audience.md
	RelComponentOnNet = "component-on-net" // component-on-net(ref_des, net): a component sits on a net. doc: facts/docs/component-on-net.md

	// net.nominal_voltage(net, volts) is the DESIGN-side nominal a rail's NAME declares (3V3 -> 3.3),
	// the same name-derived number RailMaxVoltage falls back to, but exposed on its own so a datasheet
	// range check joins the design's rail voltage as a fact rather than recomputing it in Go. Distinct
	// from net.max_voltage, which prefers an explicit max_voltage attribute over the name. (WS3-082)
	RelNetNominalVoltage = "net.nominal_voltage" // net.nominal_voltage(net, volts): name-derived rail nominal. doc: facts/docs/net.nominal_voltage.md

	// param.range(mpn, symbol, kind, min, max) is the two-sided, limit-kind-discriminated datasheet
	// relation (WS3-082). The thin param(mpn, symbol, max) carries only an upper bound and cannot tell
	// an absolute-max row from a recommended-operating one on the same symbol; param.range adds the
	// lower bound (min) and the kind token (absolute_max / recommended_operating / characteristic), so a
	// range rule (min <= nominal <= max, gated by kind) is authorable in datalog. param is kept for
	// back-compat and simple max search.
	RelParamRange = "param.range" // param.range(mpn, symbol, kind, min, max): a two-sided datasheet limit. doc: facts/docs/param.range.md

	// param.prov(mpn, symbol, doc, page, section) exposes the PROVENANCE of a datasheet parameter —
	// the SourceDoc title, the page, and the table/figure the value was read from — so "where did this
	// number come from" is a query, and a datalog-authored rule can carry the Citation onto its
	// findings (WS10-012). doc is the resolved SourceDoc title (not the raw doc_ref id), the readable
	// form a check.Citation shows. Method/confidence are not columns here (the tuple has no slot); a finding
	// gets them via check.DatasheetProvFor. Empty without --params, the same posture as param.
	RelParamProv = "param.prov" // param.prov(mpn, symbol, doc, page, section): a datasheet value's Citation. doc: facts/docs/param.prov.md

	// param.unit(mpn, symbol, unit) is the unit a parameter is PRINTED in (agni issue 165). The
	// numbers in param and param.range are reduced to SI base units so a comparison can trust them;
	// this keeps the vendor's own spelling queryable, which is what a reviewer checking a citation
	// against a datasheet page reads, and what tells apart two rows that now carry the same number.
	// String-valued, so no ordering comparison can bind it.
	RelParamUnit = "param.unit" // param.unit(mpn, symbol, unit): the unit a parameter is printed in. doc: facts/docs/param.unit.md

	// Board-geometry relations (the board tier, WS1-006): derived per-net values, not raw geometry.
	// They demonstrate the query surface is tier-general — a new tier is queryable by adding
	// projectors, no consumer change. Widths/drills are millimetres.
	RelBoardTrackWidth = "board.track_width" // board.track_width(net, mm): the net's MINIMUM copper width. doc: facts/docs/board.track_width.md
	RelBoardViaDrill   = "board.via_drill"   // board.via_drill(net, mm): the net's MINIMUM via drill. doc: facts/docs/board.via_drill.md
	RelBoardLayer      = "board.layer"       // board.layer(net, layer): a layer the net's copper occupies. doc: facts/docs/board.layer.md

	// Pin-level relations (WS3-038): the netlist tier projected at pin granularity, so a rule that
	// keys on a single pin (not just a net or component) is expressible in datalog. Every one is a
	// projection of a Model method that already computes it — no new analysis. pin.role and pin.type
	// are derived (a name-based role, an electrical-type string); pin.net is absent for an
	// unconnected pin (so `not pin.net(?r,?p,?_)` reads as "unconnected"); net.pin_count exposes the
	// net fan-out a stub-vs-real check needs; has_nc_channel is the design-level no-connect gate.
	RelPin           = "pin"             // pin(ref_des, pin): a part-type pin of a placed component. doc: facts/docs/pin.md
	RelPinRole       = "pin.role"        // pin.role(ref_des, pin, role): derived power/ground/anode/cathode. doc: facts/docs/pin.role.md
	RelPinType       = "pin.type"        // pin.type(ref_des, pin, etype): electrical type (power_in, input, ...). doc: facts/docs/pin.type.md
	RelPinNet        = "pin.net"         // pin.net(ref_des, pin, net): the net a pin is on (absent if none). doc: facts/docs/pin.net.md
	RelNetPinCount   = "net.pin_count"   // net.pin_count(net, count): connections on a net. doc: facts/docs/net.pin_count.md
	RelHasNCChannel  = "has_nc_channel"  // has_nc_channel(present): one row when the design can express no-connect. doc: facts/docs/has_nc_channel.md
	RelTypesPowerOut = "types_power_out" // types_power_out(present): one row when the source format types power-output pins (WS3-072). doc: facts/docs/types_power_out.md
	RelRail          = "rail"            // rail(net): the net is a power/ground rail (Model.IsPowerRail). doc: facts/docs/rail.md
	RelFeedback      = "feedback"        // feedback(net): the net is a regulator feedback/sense node (naming lexicon). doc: facts/docs/feedback.md
	RelComponentAttr = "component.attr"  // component.attr(ref_des, key, value): a component-level attribute. doc: facts/docs/component.attr.md

	// Device-class and net-attribute relations (WS3-074): the projections a class-quantified rule
	// needs to be authored in datalog. component.class selects a device family (one row per class tag
	// in the device_classes SET, WS3-071, so a family tag answers too); net.ground isolates the ground
	// case that rail (which covers power AND ground) cannot distinguish; net.external is the read-gap
	// marker a rule suppresses a finding on rather than firing on incomplete connectivity.
	RelComponentClass = "component.class" // component.class(ref_des, class): a device class the part is in. doc: facts/docs/component.class.md
	RelNetGround      = "net.ground"      // net.ground(net): the net is a ground rail (name-derived). doc: facts/docs/net.ground.md
	RelNetExternal    = "net.external"    // net.external(net): the net may extend onto an unread sheet. doc: facts/docs/net.external.md

	// Datasheet-derived class relation (WS3-076): the CONCEPT the esd-protection Go rule credits,
	// exposed for datalog. component.esd_rated selects a part whose seeded datasheet declares an ESD
	// rating at or above the credit floor (the same EsdRatingLimits extractor the rule uses), keyed by
	// ref_des so it joins with net.pin / component.class. Empty without a seeded set (--params), the
	// silent-by-construction posture the whole param tier has; the raw rating stays queryable via param.
	RelEsdRated = "component.esd_rated" // component.esd_rated(ref_des): part carries a floor-clearing ESD rating. doc: facts/docs/component.esd_rated.md

	// Datasheet-authoritative device class (WS10-013): the class the part's DATASHEET declares
	// (PartSpec.device_class), joined by MPN. It is the authoritative counterpart to component.class,
	// whose evidence is ref-des + description keywords: a smart high-side switch IS an eFuse because its
	// spec says so, a fact no keyword on the OrCAD export can honestly establish. One row per component
	// with a seeded, non-empty device_class; empty without a seeded set (--params), the silent-by-
	// construction posture the whole param tier has. The same value also enriches component.class's set
	// at model-build time (NewModelWithParams), so HasClass answers from it too.
	RelComponentDeviceClass = "component.device_class" // component.device_class(ref_des, class): the datasheet-declared device class. doc: facts/docs/component.device_class.md

	// bus(label, kind) surfaces the reader-detected bus constructs not yet expanded (WS1-034 Phase 1),
	// so ad-hoc bus search is expressible in datalog; label is the bus name (empty for an anonymous
	// wire), kind the source construct (bus, bus_entry, geda_bus, edif_array, xschem_bus_label, ...).
	RelBus = "bus" // doc: facts/docs/bus.md
	// RelUnresolvedSymbol is keyed by ref_des, NOT by the symbol reference, so it joins straight to
	// the components that lost pins (WS1-052). One row per affected placement, so a query can ask
	// what KIND of parts a missing library cost — the blast radius, not just the file name.
	RelUnresolvedSymbol = "unresolved_symbol" // doc: facts/docs/unresolved_symbol.md

	// Reader-diagnostic relations (WS3-081): the ENTITY-KEYED input diagnostics promoted to query
	// relations so they join to components/pins/nets (collisions on a ref-des prefix, the nets a
	// conflicted pin touches). Point-geometry diagnostics (dangling / no-junction endpoints) stay
	// rule-scoped — a bare x,y has nothing to join. The rule: a diagnostic earns a query relation when
	// it carries an entity key; `bus` (keyed by label) fits the same rule (docs/19).
	RelRefDesCollision = "ref_des_collision" // ref_des_collision(ref_des): a designator shared by >1 part. doc: facts/docs/ref_des_collision.md
	RelPinNetConflict  = "pin_net_conflict"  // pin_net_conflict(ref_des, pin, net): the read put a pin on >1 net. doc: facts/docs/pin_net_conflict.md

	// net.bus_like(net) (WS3-080): a shared-distribution net (ground plane, global-by-name rail, or
	// rail-scale fan-out) — the same predicate the series-reach walk stops at, named once and exposed
	// so "which nets are bus-scale" is a query, not a hidden constant. Distinct from bus(label,kind)
	// (WS1-034), which is a reader-detected unmodeled bus LABEL, not a high-fan-out net.
	RelNetBusLike = "net.bus_like" // doc: facts/docs/net.bus_like.md

	// Net-class relations (WS3-105): the TOOL-assigned class string a design's project file
	// records ("Default", "Power", "HighSpeed"), which is the near-universal scope expression in
	// vendor rule decks. Deliberately NOT named net.class: that name belongs to the DERIVED
	// semantic role space (ir.Net.roles from WS3-072, and the net.ground relation), and a rule
	// author who conflated the two would write a join that silently matches nothing.
	// has_netclass is the design-level presence marker, the queryable twin of check.CapNetClass:
	// only a KiCad project supplies net classes, so a netclass-SCOPED rule selects nothing on
	// every other read and reports clean. See facts/docs/net.netclass.md.
	// external_signal_net(net) (WS3-061) is the SCOPE the ESD rules share, projected so a
	// datalog-authored ESD check scopes itself exactly as the Go rules do. It is the one part of the
	// ESD guard stack that cannot be composed from existing relations: the protection predicates are
	// reachability questions and became plain datalog once reaches carried distance (WS3-112), but
	// this one reads net ATTRIBUTES (global, power_driven) and the no-connect channel, none of which
	// have a relation. Reassembling it by hand in datalog would drop a guard sooner or later, and a
	// dropped guard here is a false FAIL on a rail or an unconnected pad.
	RelExternalSignalNet = "external_signal_net" // external_signal_net(net): connector-facing signal net, the ESD scope. doc: facts/docs/external_signal_net.md

	// Derived net properties (WS3-088): what the DESIGN does, projected so it can be compared against
	// what an intent declaration says it should do — and so an engineer can ask either question ad hoc.
	// Both were private helpers inside the intent rule first; they are here because a derived predicate
	// with more than one plausible consumer belongs in the vocabulary, not inside one rule.
	RelNetBias      = "net.bias"       // net.bias(net, level): a bias resistor holds the net high or low. doc: facts/docs/net.bias.md
	RelNetACCoupled = "net.ac_coupled" // net.ac_coupled(net): a SERIES capacitor carries the net. doc: facts/docs/net.ac_coupled.md

	RelNetNetClass = "net.netclass" // net.netclass(net, class): the tool-assigned net class. doc: facts/docs/net.netclass.md
	RelHasNetClass = "has_netclass" // has_netclass(present): one row when the design assigns net classes at all. doc: facts/docs/has_netclass.md

	// Net-class DEFINITIONS (WS3-111): what the project declares a class's nets should route at,
	// keyed by CLASS. Millimetres, matching the board tier so declared and actual join with no
	// conversion. These are the raw per-class rows; the cascaded per-NET values are below.
	RelNetClassClearance   = "netclass.clearance"    // netclass.clearance(class, mm). doc: facts/docs/netclass.clearance.md
	RelNetClassTrackWidth  = "netclass.track_width"  // netclass.track_width(class, mm). doc: facts/docs/netclass.track_width.md
	RelNetClassViaDiameter = "netclass.via_diameter" // netclass.via_diameter(class, mm). doc: facts/docs/netclass.via_diameter.md
	RelNetClassViaDrill    = "netclass.via_drill"    // netclass.via_drill(class, mm). doc: facts/docs/netclass.via_drill.md
	RelHasNetClassDefs     = "has_netclass_defs"     // has_netclass_defs(present). doc: facts/docs/has_netclass_defs.md

	// The CASCADED per-net values: what a net should route at once its classes are resolved. A rule
	// comparing declared against actual joins THESE, never the per-class rows — a net in two classes
	// matches two of those, and comparing against each would fail a net that correctly obeys the
	// winning one. Only the two quantities with a board-tier counterpart are derived (WS3-111 scope).
	RelNetDeclaredTrackWidth = "net.declared_track_width" // net.declared_track_width(net, mm). doc: facts/docs/net.declared_track_width.md
	RelNetDeclaredViaDrill   = "net.declared_via_drill"   // net.declared_via_drill(net, mm). doc: facts/docs/net.declared_via_drill.md
)

// unitVolt and unitMillimetre are the BASE units the numeric relations publish, named here rather
// than spelled at each projection site so a relation cannot drift from its neighbours.
//
// MILLIMETRES ARE NOT THE SI BASE for length, and that is deliberate: mm is the unit every board
// format states and every board query is written in (`?w < 0.2`), and this field's job is to stop a
// LENGTH being compared against a VOLTAGE, not to relitigate which length unit the board tier uses.
// The invariant it must hold is that one dimension has ONE spelling across every relation, which it
// does. A datasheet length would have to be projected as mm to join, and nothing projects one today.
//
// net.pin_count and the other counts deliberately carry NO base unit: a count is dimensionless, and
// an empty base unit is polymorphic, so `?c < 5` keeps working.
const (
	unitVolt       = "V"
	unitMillimetre = "mm"
)

// Facts projects the Model into the seed fact base, deterministically ordered so the projection
// is regenerable (two calls on one Model are equal). It composes the per-relation projectors;
// a relation's facts are empty when the Model lacks that tier (a design read without a seeded
// datasheet set yields no param/mpn facts, the same silent-by-construction posture the rules
// have), so Facts never fabricates.
func Facts(m check.Model) []query.FactRow {
	var out []query.FactRow
	out = append(out, netMaxVoltageFacts(m)...)
	out = append(out, netNominalVoltageFacts(m)...)
	out = append(out, componentMPNFacts(m)...)
	out = append(out, paramFacts(m)...)
	out = append(out, paramRangeFacts(m)...)
	out = append(out, paramUnitFacts(m)...)
	out = append(out, paramProvFacts(m)...)
	out = append(out, audienceFacts(m)...)
	out = append(out, componentOnNetFacts(m)...)
	out = append(out, pinFacts(m)...)
	out = append(out, netPinCountFacts(m)...)
	out = append(out, ncChannelFacts(m)...)
	out = append(out, typesPowerOutFacts(m)...)
	out = append(out, railFacts(m)...)
	out = append(out, feedbackFacts(m)...)
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

// sortFacts orders fact rows by (relation, subject, object) for deterministic output, shared by the
// design-scoped Facts and the library-wide SpecLibFacts so both surfaces print stably.
func sortFacts(out []query.FactRow) {
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

func netMaxVoltageFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, n := range m.Nets() {
		if v, ok := check.RailMaxVoltage(n, n.Name); ok {
			vv := v
			out = append(out, query.FactRow{Relation: RelNetMaxVoltage, Subject: n.Name, Value: fmt.Sprintf("%gV", v), Num: &vv, BaseUnit: unitVolt, Cite: irCite(n.Prov)})
		}
	}
	return out
}

// netNominalVoltageFacts emits the name-derived nominal voltage of each net (3V3 -> 3.3), the
// design-side number a datasheet range check compares against. It reads only the net NAME
// (check.NominalVoltageFromName), never the max_voltage attribute — that explicit channel is
// net.max_voltage's job — so the two relations stay distinct evidence. A net whose name carries
// no parseable nominal yields no row (skip, never guess).
func netNominalVoltageFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, n := range m.Nets() {
		if v, ok := check.NominalVoltageFromName(n.Name); ok {
			vv := v
			out = append(out, query.FactRow{Relation: RelNetNominalVoltage, Subject: n.Name, Value: fmt.Sprintf("%gV", v), Num: &vv, BaseUnit: unitVolt, Cite: irCite(n.Prov)})
		}
	}
	return out
}

func componentMPNFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, c := range m.Components() {
		if mpn := m.ComponentMPN(c.RefDes); mpn != "" {
			out = append(out, query.FactRow{Relation: RelComponentMPN, Subject: c.RefDes, Value: mpn, Cite: irCite(c.Prov)})
		}
	}
	return out
}

// paramFacts emits one fact per parameter of each JOINED datasheet spec, keyed by mpn and
// deduped (several components can share one MPN, and the spec is the same). It emits every
// parameter, not only the rule-consumed ones, because the fact base is the whole datasheet: a
// rule reads a subset (cap-voltage reads the rated-voltage symbol), search reads any.
func paramFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
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

// limitKindToken renders a parameter's LimitKind as the lowercase token the query surface uses
// (absolute_max / recommended_operating / characteristic / unspecified), matching how other
// enum-ish facts are surfaced as string tokens rather than proto enum numbers.
func limitKindToken(k parampb.LimitKind) string {
	switch k {
	case parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX:
		return "absolute_max"
	case parampb.LimitKind_LIMIT_KIND_RECOMMENDED_OPERATING:
		return "recommended_operating"
	case parampb.LimitKind_LIMIT_KIND_CHARACTERISTIC:
		return "characteristic"
	default:
		return "unspecified"
	}
}

// EVERY NUMBER THE QUERY SURFACE EMITS FOR A PARAMETER IS IN ITS SI BASE UNIT (agni issue 165).
//
// A FactRow has no unit slot, so a datalog rule comparing `param.range(?m,"VDD",_,_,?max), ?max < 5.0`
// is comparing a bare number. Projected as printed, a spec seeded 4600 mV compared as 4600 against a
// 5.0 volt threshold, with no gate anywhere to refuse it. That is agni issue 148's failure on a
// surface where there is not even a unit string to gate on, so the fix is the same one: reduce
// through param.InBaseUnit, in the one place that owns the scale (C24).
//
// A row whose unit that table does not recognize keeps its symbol, kind, conditions and citation and
// has its NUMERIC slots left empty. It is not dropped, because `param` answers "what does this part
// specify" as much as it feeds a comparison, and a silently shortened list is its own quiet wrong
// answer.
//
// That is only safe because the evaluator was fixed in the same change. An absent Num used to bind a
// variable to the EMPTY STRING, and eval's comparison then fell back to string ordering, where
// "" < "5.0" is true; a row with no number would have satisfied a numeric guard rather than failed to
// match it. evalCompare now refuses to ORDER an absent number against a present one, so an
// unmeasurable value is unorderable by construction rather than by this projector omitting it. The
// same fix is what makes param.range safe to emit with one bound absent, which it does on any
// ordinary max-only datasheet row.

// specParamRows projects the `param` facts of one PartSpec — one row per parameter, keyed by mpn,
// with the upper bound in its SI base unit. Shared by the design-scoped join (paramFacts) and the
// library-wide projection (SpecLibFacts) so the two surfaces emit identical rows; the only difference
// is which specs they iterate.
func specParamRows(mpn string, spec *parampb.PartSpec) []query.FactRow {
	out := make([]query.FactRow, 0, len(spec.Parameters))
	for _, p := range spec.Parameters {
		q, ok := param.InBaseUnit(p)
		if !ok {
			// The unit has no known scale, so there is no number to publish. The row still appears,
			// carrying its symbol, conditions and citation, with the numeric slot EMPTY: an
			// unmeasurable value must not be orderable, and evalCompare refuses to order an absent
			// number against a present one.
			out = append(out, query.FactRow{Relation: RelParam, Subject: mpn, Object: p.GetSymbol(), Conditions: conditionsText(p.GetConditions()), Cite: check.Citation(spec, p)})
			continue
		}
		f := query.FactRow{Relation: RelParam, Subject: mpn, Object: q.Symbol, Value: rangeText(q.Value), BaseUnit: q.Unit, Conditions: conditionsText(q.Conditions), Cite: check.Citation(spec, p)}
		if q.Value != nil && q.Value.Max != nil {
			v := *q.Value.Max
			f.Num = &v
		}
		out = append(out, f)
	}
	return out
}

// specParamUnitRows projects the `param.unit` facts of one PartSpec: the unit each parameter is
// PRINTED in, one row per parameter.
//
// It exists because normalizing the numbers would otherwise destroy information the query surface
// used to carry. `param` and `param.range` now answer "how big is it" in a unit a comparison can
// trust; this answers "what did the vendor actually print", which is what a reviewer checking a
// citation against a datasheet page needs. Splitting them follows `param.prov`'s precedent: a
// separate relation rather than more columns on `param`, so no existing query changes arity.
//
// EVERY parameter is emitted, including one whose unit the conversion table does not recognize and
// which therefore has no row in `param` or `param.range`. This relation is the reason dropping those
// is a narrowing of the NUMERIC surface rather than a disappearance.
func specParamUnitRows(mpn string, spec *parampb.PartSpec) []query.FactRow {
	out := make([]query.FactRow, 0, len(spec.Parameters))
	for _, p := range spec.Parameters {
		out = append(out, query.FactRow{
			Relation: RelParamUnit, Subject: mpn, Object: p.GetSymbol(), Value: p.GetUnit(),
			Cite: check.Citation(spec, p),
		})
	}
	return out
}

// paramUnitFacts emits the printed unit of each joined datasheet parameter, deduped by MPN and empty
// without --params, the same silent-by-construction posture as paramFacts.
func paramUnitFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
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

// specParamRangeFacts projects the two-sided, limit-kind-discriminated view of one PartSpec: one row
// per parameter, carrying the kind token (Value), the lower bound (Min) and the upper bound (Num).
// Shared by the design-scoped join (paramRangeFacts) and the library-wide projection (SpecLibFacts).
func specParamRangeRows(mpn string, spec *parampb.PartSpec) []query.FactRow {
	out := make([]query.FactRow, 0, len(spec.Parameters))
	for _, p := range spec.Parameters {
		q, ok := param.InBaseUnit(p)
		if !ok {
			// Same posture as specParamRows: the kind and the citation are still true, the bounds are
			// not knowable, so both numeric slots stay empty rather than the row disappearing.
			out = append(out, query.FactRow{Relation: RelParamRange, Subject: mpn, Object: p.GetSymbol(), Value: limitKindToken(p.GetLimitKind()), Conditions: conditionsText(p.GetConditions()), Cite: check.Citation(spec, p)})
			continue
		}
		f := query.FactRow{Relation: RelParamRange, Subject: mpn, Object: q.Symbol, Value: limitKindToken(q.LimitKind), BaseUnit: q.Unit, Conditions: conditionsText(q.Conditions), Cite: check.Citation(spec, p)}
		if q.Value != nil {
			// BOTH bounds are reduced, and a range rule is why that matters: converting only the max
			// would leave a "3000..3.6" row, which reads as a rail far BELOW its minimum rather than
			// within range, and would fire the opposite finding.
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

// paramRangeFacts emits the two-sided, limit-kind-discriminated view of the same joined datasheet
// parameters paramFacts projects. Where param(mpn, symbol, max) exposes only the ceiling and collapses
// an absolute-max row and a recommended-operating row on one symbol into indistinguishable tuples,
// param.range keeps both bounds and the kind, so a range rule can join them apart. Deduped by MPN and
// empty without --params, the same silent-by-construction posture as paramFacts.
func paramRangeFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
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

// specParamProvRows projects the `param.prov` facts of one PartSpec — one row per parameter, carrying
// the resolved SourceDoc title (Value), the page (Num), and the table/figure (Conditions). Shared by
// the design-scoped join (paramProvFacts) and the library-wide projection (SpecLibFacts).
func specParamProvRows(mpn string, spec *parampb.PartSpec) []query.FactRow {
	out := make([]query.FactRow, 0, len(spec.Parameters))
	for _, p := range spec.Parameters {
		page := float64(p.GetProv().GetPage())
		out = append(out, query.FactRow{
			Relation:   RelParamProv,
			Subject:    mpn,
			Object:     p.Symbol,
			Value:      check.DocTitle(spec, p.GetProv().GetDocRef()),
			Num:        &page,
			Conditions: p.GetProv().GetTableOrFigure(),
			Cite:       check.Citation(spec, p),
		})
	}
	return out
}

// paramProvFacts emits the check.Citation of each joined datasheet parameter — where the value came from —
// deduped by MPN and empty without --params, the same silent-by-construction posture as paramFacts.
func paramProvFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
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

// audienceFacts projects the `part.audience` relation over the design-joined specs — one row per
// entitled team/license (param.Audience, WS10-010). Record-only; nothing enforces it yet (WS10-011).
func audienceFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
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

// audienceRows projects one part's `part.audience` facts (one per entitled identifier). Shared by the
// design-scoped and library-wide surfaces. A part with no audience annotation emits nothing.
func audienceRows(mpn string, spec *parampb.PartSpec) []query.FactRow {
	var out []query.FactRow
	for _, who := range param.Audience(spec) {
		out = append(out, query.FactRow{Relation: RelPartAudience, Subject: mpn, Object: who})
	}
	return out
}

// SpecLibFacts projects the datalog fact base of a whole seeded corpus — every PartSpec's `param`,
// `param.range` and `part.audience` rows — with NO design join (WS10-010). It is the library-wide
// analogue of Facts: where Facts derives facts for the parts ON a design, SpecLibFacts derives them for
// the parts IN the spec library, so `agni query --speclib` searches the corpus (a design is not required). Rows
// are sorted for stable output, matching Facts' ordering.
func SpecLibFacts(specs []*parampb.PartSpec) []query.FactRow {
	var out []query.FactRow
	for _, spec := range specs {
		if spec.GetMpn() == "" {
			continue
		}
		out = append(out, specParamRows(spec.GetMpn(), spec)...)
		out = append(out, specParamRangeRows(spec.GetMpn(), spec)...)
		out = append(out, specParamUnitRows(spec.GetMpn(), spec)...)
		out = append(out, specParamProvRows(spec.GetMpn(), spec)...)
		out = append(out, audienceRows(spec.GetMpn(), spec)...)
	}
	sortFacts(out)
	return out
}

func componentOnNetFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, n := range m.Nets() {
		for _, conn := range n.Connections {
			out = append(out, query.FactRow{Relation: RelComponentOnNet, Subject: conn.ComponentRef, Object: n.Name, Cite: irCite(n.Prov)})
		}
	}
	return out
}

// pinFacts projects every part-type pin of every placed component at pin granularity: its
// existence, derived role, electrical type, and the net it lands on. Role is emitted only when
// derived (check.RoleUnknown is omitted, never guessed); pin.net is omitted for an unconnected pin, so
// its ABSENCE is the queryable signal. Empty when the source carries no part-pin data (a bare
// netlist), the same silent-by-construction posture the other tiers have.
func pinFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, p := range m.Pins() {
		ref, des := p.Component.RefDes, p.Designator
		cite := irCite(p.Component.Prov)
		out = append(out, query.FactRow{Relation: RelPin, Subject: ref, Object: des, Cite: cite})
		if role := m.PinRole(ref, des); role != check.RoleUnknown {
			out = append(out, query.FactRow{Relation: RelPinRole, Subject: ref, Object: des, Value: string(role), Cite: cite})
		}
		out = append(out, query.FactRow{Relation: RelPinType, Subject: ref, Object: des, Value: check.DirString(m.PinDir(ref, des)), Cite: cite})
		if net := m.PinNetName(ref, des); net != "" {
			out = append(out, query.FactRow{Relation: RelPinNet, Subject: ref, Object: des, Value: net, Cite: cite})
		}
	}
	return out
}

// netPinCountFacts emits each net's connection count, the fan-out a rule needs to tell a
// single-pin stub net (a pin wired to nothing) from a real multi-pin net.
func netPinCountFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, n := range m.Nets() {
		c := float64(len(n.Connections))
		out = append(out, query.FactRow{Relation: RelNetPinCount, Subject: n.Name, Num: &c, Cite: irCite(n.Prov)})
	}
	return out
}

// ncChannelFacts emits a single row when the design can express intentional no-connect (a
// NO_CONNECT-typed pin or an nc-marker net), so a rule can gate on it as `has_nc_channel(?_)`.
// Absent (zero rows) otherwise, so the gate fails closed on a format that cannot express intent.
func ncChannelFacts(m check.Model) []query.FactRow {
	if m.HasNoConnectChannel() {
		return []query.FactRow{{Relation: RelHasNCChannel, Subject: "true", Cite: "design"}}
	}
	return nil
}

// typesPowerOutFacts emits one row when the source format classifies power-OUTPUT pins (KiCad/gEDA do,
// EDIF/IPC do not — see Model.FormatTypesPowerOut). The queryable twin of the design.types_power_out
// spec fact power-input-not-driven gates on, so "can I trust a driver-absence check on this design" is
// answerable from `agni query`, the same shape as has_nc_channel.
func typesPowerOutFacts(m check.Model) []query.FactRow {
	if m.FormatTypesPowerOut() {
		return []query.FactRow{{Relation: RelTypesPowerOut, Subject: "true", Cite: "design"}}
	}
	return nil
}

// railFacts emits one row per net that is a power or ground rail (Model.IsPowerRail: asserted-driven,
// global, or rail-named). It lets a datalog rule ask "does this signal reach a rail" —
// reaches(?sig, ?r), rail(?r) — the shape an interface profile's pull-up check needs.
func railFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, n := range m.Nets() {
		if m.IsPowerRail(n.Name) {
			out = append(out, query.FactRow{Relation: RelRail, Subject: n.Name, Cite: irCite(n.Prov)})
		}
	}
	return out
}

// feedbackFacts emits one row per net the naming lexicon reads as a regulator feedback / sense node
// (WS3-069/067). It is the datalog equivalent of the test-point rule's feedback exclusion: a datalog
// rule can now ask "a rail that is not a feedback node" — rail(?n), not feedback(?n).

func feedbackFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, n := range m.Nets() {
		if check.NetHasRole(n, check.NetRoleFeedback, m.IsFeedbackName) {
			out = append(out, query.FactRow{Relation: RelFeedback, Subject: n.Name, Cite: irCite(n.Prov)})
		}
	}
	return out
}

// componentAttrFacts emits each component-level attribute as component.attr(ref, key, value). It
// lets a datalog rule identify a part by a declared property — an interface profile binds its host
// this way (component.attr(?ref, "interface", "SPI_NOR")), the annotation path that removes net-name
// guessing. Empty when the source carries no component attributes.
func componentAttrFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, c := range m.Components() {
		for k, v := range c.Attributes {
			out = append(out, query.FactRow{Relation: RelComponentAttr, Subject: c.RefDes, Object: k, Value: v, Cite: irCite(c.Prov)})
		}
	}
	return out
}

// componentClassFacts emits component.class(ref, class) once per class tag in a component's
// device_classes set (WS3-071 widened the WS3-074 relation from the single most-specific class to the
// set), so it returns MULTIPLE rows for a part with a family tag: a TVS answers both
// component.class(D1, "tvs") and component.class(D1, "diode"), and a datalog rule asks family
// membership by joining on the family tag. The class string is the canonical lowercase name
// (crystal, capacitor, resistor, ...). Empty for an unclassified component (no tag is guessed).
func componentClassFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, c := range m.Components() {
		for _, cl := range m.Classes(c.RefDes) {
			out = append(out, query.FactRow{Relation: RelComponentClass, Subject: c.RefDes, Value: string(cl), Cite: irCite(c.Prov)})
		}
	}
	return out
}

// esdRatedFacts emits component.esd_rated(ref) for each component whose joined datasheet spec carries
// an ESD rating at or above the credit floor (check.EsdRatingLimits, the same extractor esd-protection's Go
// rule uses). Keyed by ref_des so a datalog rule joins it against net.pin / component.class; the
// check.Citation is the datasheet ESD row (the real evidence), not the component's IR site. Empty when the
// Model has no seeded params (m.PartSpec nil for every ref), the param tier's silent-by-construction posture.
func esdRatedFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, c := range m.Components() {
		spec := m.PartSpec(c.RefDes)
		if spec == nil {
			continue
		}
		limits := check.EsdRatingLimits(spec)
		if len(limits) == 0 {
			continue
		}
		out = append(out, query.FactRow{Relation: RelEsdRated, Subject: c.RefDes, Cite: check.Citation(spec, limits[0])})
	}
	return out
}

// componentDeviceClassFacts emits component.device_class(ref, class) for each component whose joined
// datasheet spec declares a non-empty device_class (WS10-013). The check.Citation is the spec's source
// document (device_class is a PartSpec-level field, so there is no per-parameter provenance to cite).
// Empty when the Model has no seeded params (m.PartSpec nil for every ref), the param tier's
// silent-by-construction posture.
//
// The value is NORMALIZED through classify.NormalizeDeviceClass (WS3-044), which folds vendor spelling
// variants onto one canonical key ("Ceramic Resonator" and "ceramic resonator" both reach `resonator`)
// and passes an unrecognized-but-meaningful value through unchanged, so nothing is lost.
//
// It used to project verbatim, which made this relation disagree with the OTHER consumer of the same
// field: check.enrichClassesFromParams already normalizes before merging device_class into a
// component's class set, so `component.class` answered on the canonical key while
// `component.device_class` answered on the raw one. Anything matching an exact string across the two —
// a profile binding its host by class, WS3-044 — would have had to know which of the two it was
// talking to. Normalizing here is what lets a declared class match without the author guessing the
// vendor's capitalization.
func componentDeviceClassFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, c := range m.Components() {
		spec := m.PartSpec(c.RefDes)
		if spec == nil || spec.GetDeviceClass() == "" {
			continue
		}
		cl := string(classify.NormalizeDeviceClass(spec.GetDeviceClass()))
		out = append(out, query.FactRow{Relation: RelComponentDeviceClass, Subject: c.RefDes, Value: cl, Cite: specDocCite(spec)})
	}
	return out
}

// specDocCite renders a spec-level check.Citation (the first source document's title) for a PartSpec fact that
// has no per-parameter provenance, e.g. the device_class field. "" resolves to "unknown source", the
// same rendering check.Citation() uses for a missing doc.
func specDocCite(spec *parampb.PartSpec) string {
	doc := "unknown source"
	if docs := spec.GetDocs(); len(docs) > 0 && docs[0].GetTitle() != "" {
		doc = docs[0].GetTitle()
	}
	return fmt.Sprintf("datasheet %q", doc)
}

// netGroundFacts emits net.ground(net) for each ground-named net. The rail relation covers BOTH
// power and ground (Model.IsPowerRail ORs the ground test), so this isolates the ground case a rule
// must treat differently from a supply rail — e.g. a grounded crystal case pin is not the Vdd pin
// of an active oscillator, so a datalog rule reads `rail(?r), not net.ground(?r)` for "supply rail".
func netGroundFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, n := range m.Nets() {
		if m.IsGroundNet(n) {
			out = append(out, query.FactRow{Relation: RelNetGround, Subject: n.Name, Cite: irCite(n.Prov)})
		}
	}
	return out
}

// netExternalFacts emits net.external(net) for each net the read flagged as possibly extending onto
// an unread sheet (netgraph.AttrExternal). It lets a datalog rule SUPPRESS a finding on a read-gap
// net rather than fire on incomplete connectivity — the external-skip the decoupling/bulk-cap and
// crystal rules apply in Go. Empty when the read is complete (no external nets), so the guard is a
// no-op on a fully-resolved design.
func netExternalFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, n := range m.Nets() {
		if n.GetAttributes()[netgraph.AttrExternal] == "true" {
			out = append(out, query.FactRow{Relation: RelNetExternal, Subject: n.Name, Cite: irCite(n.Prov)})
		}
	}
	return out
}

// busFacts emits bus(label, kind) for each reader-detected unmodeled bus (WS1-034 Phase 1), so a
// datalog query can list or filter buses (e.g. bus(?l, "geda_bus")). label is the source bus name,
// empty for an anonymous bus wire; kind is the construct. Empty for a design with no bus.
func busFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, b := range m.UnmodeledBuses() {
		out = append(out, query.FactRow{Relation: RelBus, Subject: b.GetLabel(), Value: b.GetKind(), Cite: irCite(b.GetProv())})
	}
	return out
}

// unresolvedSymbolFacts emits unresolved_symbol(ref_des, symref) once per PLACEMENT that lost its
// pins (WS1-052). Keyed by ref_des rather than by the reference, because a ref_des is what every
// other netlist relation joins on: `unresolved_symbol(?r, ?sym), component.class(?r, "fpga")` asks
// whether anything IMPORTANT lost its pins, which the file name alone cannot answer. A design whose
// symbols all resolved emits nothing.
func unresolvedSymbolFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, u := range m.UnresolvedSymbols() {
		for _, ref := range u.GetRefDes() {
			out = append(out, query.FactRow{Relation: RelUnresolvedSymbol, Subject: ref, Value: u.GetSymref(), Cite: irCite(u.GetProv())})
		}
	}
	return out
}

// refDesCollisionFacts emits ref_des_collision(ref) for each designator used by more than one part
// (WS3-081), keyed by ref_des so a query joins it to components (e.g. collisions on a ref-des prefix).
// The check.Citation is the first colliding instance. Empty for a design with no collision.
func refDesCollisionFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, c := range m.RefDesCollisions() {
		cite := ""
		if len(c.Instances) > 0 {
			cite = irCite(c.Instances[0])
		}
		out = append(out, query.FactRow{Relation: RelRefDesCollision, Subject: c.GetRefDes(), Cite: cite})
	}
	return out
}

// pinNetConflictFacts emits pin_net_conflict(ref, pin, net) once PER net a pin was placed on when the
// read put a single pin on more than one net (WS3-081, the integrity tripwire). The multi-row shape
// lets a query find every net a conflicted pin touches and join to those nets. Empty when the read is clean.
func pinNetConflictFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, pc := range m.PinNetConflicts() {
		for _, net := range pc.Nets {
			out = append(out, query.FactRow{Relation: RelPinNetConflict, Subject: pc.RefDes, Object: pc.Pin, Value: net, Cite: irCite(pc.Prov)})
		}
	}
	return out
}

// netBusLikeFacts emits net.bus_like(net) for each shared-distribution net (WS3-080), reusing the
// exact check.IsBusLike predicate the series-reach walk stops at, so the two share one definition. Empty
// for a design of only point-to-point nets.
func netBusLikeFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, n := range m.Nets() {
		if check.IsBusLike(m, n) {
			out = append(out, query.FactRow{Relation: RelNetBusLike, Subject: n.Name, Cite: irCite(n.Prov)})
		}
	}
	return out
}

// externalSignalNetFacts emits external_signal_net(net) for each connector-facing signal net, the
// scope check.ExternalSignalNet defines and the two ESD rules share. One row per in-scope net; empty
// on a design with no connectors, which is the honest answer rather than a permissive one — an ESD
// question about a board that exposes nothing has nothing to ask about.
func externalSignalNetFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, n := range m.Nets() {
		if check.ExternalSignalNet(m, n) {
			out = append(out, query.FactRow{Relation: RelExternalSignalNet, Subject: n.Name, Cite: irCite(n.Prov)})
		}
	}
	return out
}

// netBiasFacts emits net.bias(net, "high"|"low") for each net a bias resistor holds at a rail. A net
// with no bias, or with a divider holding it at neither rail, yields no row — so `not net.bias(?n,?_)`
// reads as "unbiased", which is a genuinely different state from "biased the other way".
func netBiasFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
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
		out = append(out, query.FactRow{Relation: RelNetBias, Subject: n.Name, Value: level, Cite: irCite(n.Prov)})
	}
	return out
}

// netACCoupledFacts emits net.ac_coupled(net) for each net a SERIES capacitor carries. A decoupling
// cap (far side on ground or a rail) does not count — that distinction is the whole predicate, since
// both uses are "a capacitor on the net".
func netACCoupledFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, n := range m.Nets() {
		if check.ACCoupled(m, n) {
			out = append(out, query.FactRow{Relation: RelNetACCoupled, Subject: n.Name, Cite: irCite(n.Prov)})
		}
	}
	return out
}

// netNetClassFacts emits net.netclass(net, class) for each class a net belongs to. The value is the
// string the design tool recorded verbatim, not a derived role, so a query scopes by the same label
// the layout engineer sees in KiCad. ONE ROW PER (net, class) PAIR, so a net in two classes fans out
// to two rows and `?net` is not unique in this projection (WS1-050) — the same 1:many shape
// component.class has. Nets left in the tool's implicit default carry no class and yield no row, so
// `not net.netclass(?n, ?_)` reads as "unclassed". Empty for every source but a KiCad project read
// — see hasNetClassFacts.
func netNetClassFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, n := range m.Nets() {
		for _, c := range n.NetClasses {
			out = append(out, query.FactRow{Relation: RelNetNetClass, Subject: n.Name, Value: c, Cite: irCite(n.Prov)})
		}
	}
	return out
}

// hasNetClassFacts emits the single has_netclass(true) row when the design assigns net classes at
// all (Model.HasNetClasses). It is the queryable twin of check.CapNetClass, the same shape
// typesPowerOutFacts has for CapTypesPowerOut: a rule scoped by net class must be able to tell
// "no net is in class HV" from "this design has no classes", and absent the marker those are the
// same empty result.
func hasNetClassFacts(m check.Model) []query.FactRow {
	if m.HasNetClasses() {
		return []query.FactRow{{Relation: RelHasNetClass, Subject: "true", Cite: "design"}}
	}
	return nil
}

// boardFacts projects the board tier (WS1-006): per net, the MINIMUM copper track width and via
// drill (the safety-relevant extreme, in mm — not every raw segment) and the layers the net's
// copper occupies. Empty when the Model has no board tier (a netlist-only design), the same
// silent-by-construction posture the params tier has. Cite is a descriptive board reference: a
// derived BoardNet carries no file/span, and the net's copper is what a reader inspects to verify.
func boardFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, bn := range m.BoardNets() {
		cite := "board net " + bn.Net
		if w, ok := minSegmentWidthNm(bn.Segments); ok {
			mm := nmToMM(w)
			out = append(out, query.FactRow{Relation: RelBoardTrackWidth, Subject: bn.Net, Value: mmStr(mm), Num: &mm, BaseUnit: unitMillimetre, Cite: cite})
		}
		if d, ok := minViaDrillNm(bn.Vias); ok {
			mm := nmToMM(d)
			out = append(out, query.FactRow{Relation: RelBoardViaDrill, Subject: bn.Net, Value: mmStr(mm), Num: &mm, BaseUnit: unitMillimetre, Cite: cite})
		}
		for _, layer := range netLayers(bn.Segments) {
			out = append(out, query.FactRow{Relation: RelBoardLayer, Subject: bn.Net, Object: layer, Cite: cite})
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

// nmToMM converts a board dimension from nanometres to millimetres — the human-natural unit the
// board relations expose, so a query reads `?w < 0.2` (mm) not `?w < 200000` (nm).
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

// netClassDefFacts emits the RAW per-class declarations: one row per (class, quantity) the project
// actually stated. A class that declares no track width yields no track-width row, which is the
// fact a consumer needs — that field cascades to a lower-priority class rather than being zero.
func netClassDefFacts(m check.Model) []query.FactRow {
	var out []query.FactRow
	for _, c := range m.NetClassDefs() {
		for _, p := range netClassDefParams {
			mm, ok := parseMM(c.GetParams()[p.param])
			if !ok {
				continue
			}
			v := mm
			out = append(out, query.FactRow{Relation: p.rel, Subject: c.GetName(), Value: mmStr(v), Num: &v, BaseUnit: unitMillimetre, Cite: "net_settings"})
		}
	}
	return out
}

// hasNetClassDefsFacts is the design-level marker for DEFINITIONS, the twin of has_netclass for
// membership. The two are genuinely independent: net_settings carries assignments and class
// definitions in separate blocks, so a project can assign nets to classes it never defines. A
// declared-vs-actual rule that found no definitions would report clean, so it gates on this.
func hasNetClassDefsFacts(m check.Model) []query.FactRow {
	if len(m.NetClassDefs()) == 0 {
		return nil
	}
	return []query.FactRow{{Relation: RelHasNetClassDefs, Subject: "true", Cite: "design"}}
}

// netDeclaredFacts resolves each net's EFFECTIVE declared values and emits one row per net per
// quantity. This is the cascade, and it is the reason the raw per-class rows are not what a rule
// should join.
//
// KiCad composes an effective netclass PER FIELD, not per class: it sorts a net's constituent
// classes by priority ascending (the Default class pinned last) and fills each field from the first
// class that states it. So a net in a high-priority class declaring only a clearance still takes its
// track width from the next class down. There is no single winning class, and picking one would be
// wrong in a way that produces confident, incorrect findings.
//
// A net whose classes state a quantity nowhere yields no row for it, so a rule joining this relation
// selects only nets the project actually constrained.
func netDeclaredFacts(m check.Model) []query.FactRow {
	defs := m.NetClassDefs()
	if len(defs) == 0 {
		return nil
	}
	byName := make(map[string]*ir.Constraint, len(defs))
	for _, c := range defs {
		byName[c.GetName()] = c
	}
	// The default class is not merely the lowest-priority one: it applies to EVERY net, filling
	// whatever that net's own classes left unstated, and a net in no class takes its values
	// outright. A cascade over memberships alone would under-report on both counts.
	var defaultClass string
	for _, c := range defs {
		if c.GetParams()["is_default"] == "true" {
			defaultClass = c.GetName()
			break
		}
	}

	var out []query.FactRow
	for _, n := range m.Nets() {
		classes := append([]string(nil), n.GetNetClasses()...)
		// Cascade order is the project's priority, NOT the net's (alphabetical) membership order.
		sort.SliceStable(classes, func(i, j int) bool {
			return defPriority(byName[classes[i]]) < defPriority(byName[classes[j]])
		})
		if defaultClass != "" && !slices.Contains(classes, defaultClass) {
			classes = append(classes, defaultClass) // always last, always present
		}
		for _, q := range []struct {
			param string
			rel   string
		}{
			{"track_width", RelNetDeclaredTrackWidth},
			{"via_drill", RelNetDeclaredViaDrill},
		} {
			for _, cls := range classes {
				mm, ok := parseMM(byName[cls].GetParams()[q.param])
				if !ok {
					continue // this class does not state it; fall through to the next
				}
				v := mm
				out = append(out, query.FactRow{
					Relation: q.rel, Subject: n.GetName(), Value: mmStr(v), Num: &v, BaseUnit: unitMillimetre,
					Cite: "net_settings:" + cls,
				})
				break // first stating class wins for THIS field only
			}
		}
	}
	return out
}

// defPriority reads a class's cascade rank. An unknown class (a net assigned to a class the project
// never defined, which net_settings permits) sorts last rather than first: it states nothing, so it
// must never outrank a class that does.
func defPriority(c *ir.Constraint) int {
	if c == nil {
		return math.MaxInt32
	}
	p, err := strconv.Atoi(c.GetParams()["priority"])
	if err != nil {
		return math.MaxInt32
	}
	return p
}

// parseMM reads a declared millimetre scalar. Absent and unparseable both read as "not stated",
// which is the safe direction: a value we cannot read must not become a limit we compare against.
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

func nmToMM(nm int64) float64 { return float64(nm) / 1e6 }

func mmStr(mm float64) string { return fmt.Sprintf("%gmm", mm) }

// irCite renders an IR provenance as a one-line source check.Citation: the source file, narrowed by
// the reader's native id when present (the addressable unit a viewer can navigate to).
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
