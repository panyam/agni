package query

import (
	"sort"

	"github.com/panyam/agni/core/facts"
)

// RelationInfo describes one queryable relation or predicate for discovery surfaces. It is an alias
// for the fact layer's type, because a picker showing relations and predicates in one list must not
// have to reconcile two shapes for the same row.
type RelationInfo = facts.RelationInfo

// Relation kinds and their display order, re-exported so a caller rendering this engine's catalog
// need not import the fact layer for the grouping labels alone.
const (
	KindNetlist   = facts.KindNetlist
	KindBoard     = facts.KindBoard
	KindDatasheet = facts.KindDatasheet
	KindPredicate = facts.KindPredicate
	KindOverlay   = facts.KindOverlay
)

// KindOrder is the display order of the kind groups, most-common first.
var KindOrder = facts.KindOrder

// builtinPredicates is the human-facing metadata for this engine's COMPUTED built-in predicates —
// reaches (a Model-driven generator) and the string filters. These are evaluator primitives, not
// fact-base relations, which is why their metadata lives here while the relations' lives with the
// relations (facts.Relations). Arg counts are asserted against the schema in TestCatalogMatchesSchema,
// so a predicate added without a catalog entry — or with a mismatched arity — fails CI rather than
// shipping undiscoverable.
var builtinPredicates = []RelationInfo{
	{Name: "reaches", Args: []string{"from", "net", "hops?"}, Summary: "transitive reachability through series pass elements (R/L/ferrite/fuse); the optional third argument binds the EXACT number of crossings, so a radius is written `reaches(?a,?b,?h), ?h <= 2` and not `reaches(?a,?b,2)`, which means exactly two", Kind: KindPredicate},
	{Name: "contains", Args: []string{"string", "substring"}, Summary: "the string contains the substring", Kind: KindPredicate},
	{Name: "prefix", Args: []string{"string", "prefix"}, Summary: "the string starts with the prefix", Kind: KindPredicate},
	{Name: "suffix", Args: []string{"string", "suffix"}, Summary: "the string ends with the suffix", Kind: KindPredicate},
	{Name: "glob", Args: []string{"string", "pattern"}, Summary: "the whole string matches a shell-style glob (* any run, ? one char)", Kind: KindPredicate},
	{Name: "match", Args: []string{"string", "regex"}, Summary: "the string matches an (unanchored) regular expression", Kind: KindPredicate},
	{Name: "absent", Args: []string{"value"}, Summary: "the field carried no value at all, which is different from an empty string and from zero (a datasheet row stating only a maximum leaves its minimum absent); `not absent(?x)` reads \"this row states one\"", Kind: KindPredicate},
}

// Catalog returns this engine's discoverable construct set: every fact-base relation (built-in and
// overlay-registered, from facts.Relations) plus the predicates the evaluator computes. The result is
// sorted by kind (KindOrder) then name, so a caller renders a stable grouped list without re-sorting.
// Overlay predicates (RegisterPredicate) are not listed: they carry no arg-label metadata, so a
// template would be unhelpful.
func Catalog() []RelationInfo {
	rels := facts.Relations()
	out := make([]RelationInfo, 0, len(rels)+len(builtinPredicates))
	out = append(out, rels...)
	out = append(out, builtinPredicates...)
	// A predicate's reference markdown resolves through the same doc registry a relation's does, so a
	// documented predicate lists with its Detail and an undocumented one still lists with its Summary.
	for i := range out {
		if out[i].Detail == "" {
			out[i].Detail = facts.Doc(out[i].Name)
		}
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
