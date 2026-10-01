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
	KindExtension = facts.KindExtension
)

// KindOrder is the display order of the kind groups, most-common first.
var KindOrder = facts.KindOrder

// Catalog returns this engine's discoverable construct set: every fact-base relation and every
// predicate in the fact layer's vocabulary, the engine's own string tests included. The result is
// sorted by kind (KindOrder) then name, so a caller renders a stable grouped list without re-sorting.
func Catalog() []RelationInfo { return CatalogFrom(facts.DefaultRegistry()) }

// CatalogFrom is Catalog over an explicit relation vocabulary.
func CatalogFrom(reg *facts.Registry) []RelationInfo {
	rels := reg.Relations()
	preds := reg.Predicates()
	out := make([]RelationInfo, 0, len(rels)+len(preds))
	out = append(out, rels...)
	out = append(out, preds...)
	// A predicate's reference markdown resolves through the same doc registry a relation's does, so a
	// documented predicate lists with its Detail and an undocumented one still lists with its Summary.
	for i := range out {
		if out[i].Detail == "" {
			out[i].Detail = reg.Doc(out[i].Name)
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
