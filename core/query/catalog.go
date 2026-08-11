package query

import (
	"sort"
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

// builtinPredicates is the human-facing metadata for the query engine's COMPUTED built-in
// predicates — reaches (a Model-driven generator) and the string filters. These are engine
// primitives, not fact-base relations, so they stay in query while the built-in RELATIONS'
// metadata is registered by stdlib/relations (BuiltinFacts.Catalog, issue 10). Arg counts of the
// relations are asserted against edbSchema in the relations tests (TestCatalogMatchesSchema), so a
// relation added to the schema without a catalog entry — or with a mismatched arity — fails CI
// rather than shipping an undiscoverable relation.
var builtinPredicates = []RelationInfo{
	{Name: "reaches", Args: []string{"from", "net", "hops?"}, Summary: "transitive reachability through series pass elements (R/L/ferrite/fuse); the optional third argument binds the EXACT number of crossings, so a radius is written `reaches(?a,?b,?h), ?h <= 2` and not `reaches(?a,?b,2)`, which means exactly two", Kind: KindPredicate},
	{Name: "contains", Args: []string{"string", "substring"}, Summary: "the string contains the substring", Kind: KindPredicate},
	{Name: "prefix", Args: []string{"string", "prefix"}, Summary: "the string starts with the prefix", Kind: KindPredicate},
	{Name: "suffix", Args: []string{"string", "suffix"}, Summary: "the string ends with the suffix", Kind: KindPredicate},
	{Name: "glob", Args: []string{"string", "pattern"}, Summary: "the whole string matches a shell-style glob (* any run, ? one char)", Kind: KindPredicate},
	{Name: "match", Args: []string{"string", "regex"}, Summary: "the string matches an (unanchored) regular expression", Kind: KindPredicate},
	{Name: "absent", Args: []string{"value"}, Summary: "the field carried no value at all, which is different from an empty string and from zero (a datasheet row stating only a maximum leaves its minimum absent); `not absent(?x)` reads \"this row states one\"", Kind: KindPredicate},
}

// Catalog returns the discoverable relation set: the built-in relations and predicates, plus any
// overlay-registered relations (RegisterRelation), each with synthesized arg labels from its field
// layout. The result is sorted by kind (KindOrder) then name, so a caller renders a stable grouped
// list without re-sorting. Overlay predicates (RegisterPredicate) are not listed: they carry no
// arg-label metadata, so a template would be unhelpful.
func Catalog() []RelationInfo {
	out := make([]RelationInfo, 0, len(builtinRelationCatalog)+len(builtinPredicates)+len(registryOrder))
	out = append(out, builtinRelationCatalog...) // built-in relations, registered by stdlib/relations
	out = append(out, builtinPredicates...)      // engine-computed predicates
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
		out[i].Detail = relationDoc(out[i].Name)
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
