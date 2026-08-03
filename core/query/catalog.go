package query

import (
	"sort"

	"github.com/panyam/agni/core/check"
)

// RelationInfo describes one queryable relation or predicate for discovery surfaces (the web query
// panel's relation picker today; a CLI listing or an LSP later): its name, the argument labels a
// template inserts as `?arg`, a one-line summary, and a Kind for grouping. It is metadata only —
// the authoritative behavior stays in edbSchema (layout), the projectors (facts), and builtins
// (predicates); this is the human-facing catalog over them.
type RelationInfo struct {
	Name    string
	Args    []string
	Summary string
	Kind    string // "netlist" | "board" | "datasheet" | "predicate" | "overlay"
	// Detail is the relation's rich reference markdown from check/facts/docs/<name>.md (WS14-005),
	// or "" when the relation has no doc yet (the staged backfill: not every relation is documented
	// on day one). It is the deep-dive behind Summary — a discovery surface shows Summary in a list
	// and Detail on demand. Populated by Catalog(); it is not part of the arity-vs-schema assertion.
	Detail string
}

// Relation kinds. A picker groups by these and orders the groups netlist → board → datasheet →
// predicate → overlay (KindOrder).
const (
	KindNetlist   = "netlist"
	KindBoard     = "board"
	KindDatasheet = "datasheet"
	KindPredicate = "predicate"
	KindOverlay   = "overlay"
)

// KindOrder is the display order of the kind groups, most-common first.
var KindOrder = []string{KindNetlist, KindBoard, KindDatasheet, KindPredicate, KindOverlay}

// builtinCatalog is the human-facing metadata for the built-in relations and predicates. Arg counts
// are asserted against edbSchema in the tests (TestCatalogMatchesSchema), so a relation added to the
// schema without a catalog entry — or with a mismatched arity — fails CI rather than shipping an
// undiscoverable relation.
var builtinCatalog = []RelationInfo{
	{Name: "component.mpn", Args: []string{"ref_des", "mpn"}, Summary: "the design-side part identity (manufacturer part number)", Kind: KindNetlist},
	{Name: "component-on-net", Args: []string{"ref_des", "net"}, Summary: "a component sits on a net", Kind: KindNetlist},
	{Name: "net.max_voltage", Args: []string{"net", "volts"}, Summary: "a net's declared rail voltage", Kind: KindNetlist},
	{Name: "net.nominal_voltage", Args: []string{"net", "volts"}, Summary: "a rail's nominal voltage derived from its net name (3V3 -> 3.3)", Kind: KindNetlist},
	{Name: "board.layer", Args: []string{"net", "layer"}, Summary: "a net appears on a board copper layer", Kind: KindBoard},
	{Name: "board.track_width", Args: []string{"net", "mm"}, Summary: "a copper track's width on a net (millimetres)", Kind: KindBoard},
	{Name: "board.via_drill", Args: []string{"net", "mm"}, Summary: "a via's drill diameter on a net (millimetres)", Kind: KindBoard},
	{Name: "pin", Args: []string{"ref_des", "pin"}, Summary: "a part-type pin of a placed component", Kind: KindNetlist},
	{Name: "pin.role", Args: []string{"ref_des", "pin", "role"}, Summary: "a pin's derived role (power/ground/anode/cathode)", Kind: KindNetlist},
	{Name: "pin.type", Args: []string{"ref_des", "pin", "etype"}, Summary: "a pin's electrical type (power_in, input, output, ...)", Kind: KindNetlist},
	{Name: "pin.net", Args: []string{"ref_des", "pin", "net"}, Summary: "the net a pin is on (absent if unconnected)", Kind: KindNetlist},
	{Name: "net.pin_count", Args: []string{"net", "count"}, Summary: "the number of connections on a net", Kind: KindNetlist},
	{Name: "has_nc_channel", Args: []string{"present"}, Summary: "one row when the design can express intentional no-connect", Kind: KindNetlist},
	{Name: "types_power_out", Args: []string{"present"}, Summary: "one row when the source format classifies power-output pins (EDIF/IPC do not, so a driver-absence check is unsound there)", Kind: KindNetlist},
	{Name: "rail", Args: []string{"net"}, Summary: "the net is a power or ground rail", Kind: KindNetlist},
	{Name: "feedback", Args: []string{"net"}, Summary: "the net is a regulator feedback / sense node (must not be probed)", Kind: KindNetlist},
	{Name: "component.attr", Args: []string{"ref_des", "key", "value"}, Summary: "a component-level attribute (e.g. interface, MPN)", Kind: KindNetlist},
	{Name: "component.class", Args: []string{"ref_des", "class"}, Summary: "a device class the part is in (a family tag too, e.g. a TVS is both tvs and diode)", Kind: KindNetlist},
	{Name: "component.esd_rated", Args: []string{"ref_des"}, Summary: "the part carries a datasheet ESD rating at or above the credit floor (needs --params)", Kind: KindDatasheet},
	{Name: "component.device_class", Args: []string{"ref_des", "class"}, Summary: "the device class the part's datasheet declares (authoritative over the ref-des/keyword class; needs --params)", Kind: KindDatasheet},
	{Name: "net.ground", Args: []string{"net"}, Summary: "the net is a ground rail (name-derived)", Kind: KindNetlist},
	{Name: "net.external", Args: []string{"net"}, Summary: "the net may extend onto an unread sheet (read-gap marker)", Kind: KindNetlist},
	{Name: "bus", Args: []string{"label", "kind"}, Summary: "a reader-detected bus not yet expanded into member nets (WS1-034)", Kind: KindNetlist},
	{Name: "ref_des_collision", Args: []string{"ref_des"}, Summary: "a reference designator used by more than one part (reader integrity diagnostic)", Kind: KindNetlist},
	{Name: "pin_net_conflict", Args: []string{"ref_des", "pin", "net"}, Summary: "a pin the read placed on more than one net; one row per net (reader integrity diagnostic)", Kind: KindNetlist},
	{Name: "net.bus_like", Args: []string{"net"}, Summary: "a shared-distribution net (ground plane, global rail, or rail-scale fan-out) — the series-reach walk's stop predicate", Kind: KindNetlist},
	{Name: "param", Args: []string{"mpn", "symbol", "max"}, Summary: "a datasheet parameter's max value for a part (needs --params)", Kind: KindDatasheet},
	{Name: "param.range", Args: []string{"mpn", "symbol", "kind", "min", "max"}, Summary: "a datasheet parameter's two-sided limit with its kind (absolute_max / recommended_operating / characteristic; needs --params)", Kind: KindDatasheet},
	{Name: "param.prov", Args: []string{"mpn", "symbol", "doc", "page", "section"}, Summary: "the citation of a datasheet parameter — the SourceDoc title, page, and table/figure it was read from (needs --params)", Kind: KindDatasheet},
	{Name: "part.audience", Args: []string{"mpn", "who"}, Summary: "a team/license entitled to see a part's datasheet data (record-only, needs --params)", Kind: KindDatasheet},
	{Name: "reaches", Args: []string{"from", "net"}, Summary: "transitive reachability through series pass elements (R/L/ferrite/fuse)", Kind: KindPredicate},
	{Name: "contains", Args: []string{"string", "substring"}, Summary: "the string contains the substring", Kind: KindPredicate},
	{Name: "prefix", Args: []string{"string", "prefix"}, Summary: "the string starts with the prefix", Kind: KindPredicate},
	{Name: "suffix", Args: []string{"string", "suffix"}, Summary: "the string ends with the suffix", Kind: KindPredicate},
}

// Catalog returns the discoverable relation set: the built-in relations and predicates, plus any
// overlay-registered relations (RegisterRelation), each with synthesized arg labels from its field
// layout. The result is sorted by kind (KindOrder) then name, so a caller renders a stable grouped
// list without re-sorting. Overlay predicates (RegisterPredicate) are not listed: they carry no
// arg-label metadata, so a template would be unhelpful.
func Catalog() []RelationInfo {
	out := make([]RelationInfo, 0, len(builtinCatalog)+len(registryOrder))
	out = append(out, builtinCatalog...)
	for _, name := range registryOrder {
		def := registry[name]
		args := make([]string, len(def.fields))
		for i, f := range def.fields {
			args[i] = fieldLabel(f)
		}
		out = append(out, RelationInfo{Name: name, Args: args, Summary: "overlay-registered relation", Kind: KindOverlay})
	}
	// Attach the rich reference markdown (WS14-005) where a relation has a doc; "" otherwise, so an
	// undocumented relation still lists with its Summary.
	for i := range out {
		out[i].Detail = check.RelationDoc(out[i].Name)
	}
	kindRank := map[string]int{}
	for i, k := range KindOrder {
		kindRank[k] = i
	}
	sort.SliceStable(out, func(i, j int) bool {
		if ri, rj := kindRank[out[i].Kind], kindRank[out[j].Kind]; ri != rj {
			return ri < rj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// fieldLabel names a FactRow field for an overlay relation's synthesized template arg.
func fieldLabel(f edbField) string {
	switch f {
	case fSubject:
		return "subject"
	case fObject:
		return "object"
	case fValue:
		return "value"
	case fNum:
		return "n"
	case fConditions:
		return "conditions"
	}
	return "arg"
}
