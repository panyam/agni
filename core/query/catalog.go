package query

import (
	"sort"

	"github.com/panyam/agni/core/facts"
	"github.com/panyam/jaala/datalog"
	"github.com/panyam/jaala/ns"
)

// RelationInfo describes one queryable relation or predicate for discovery surfaces. It is an alias
// for the fact layer's type. A picker shows relations and predicates in one list, and with one type
// it never has to reconcile two shapes for the same row.
type RelationInfo = facts.RelationInfo

// Relation kinds and their display order, re-exported so a caller can render this engine's catalog
// without importing the fact layer for the grouping labels alone.
const (
	KindNetlist   = facts.KindNetlist
	KindBoard     = facts.KindBoard
	KindDatasheet = facts.KindDatasheet
	KindPredicate = facts.KindPredicate
	KindDerived   = facts.KindDerived
	KindExtension = facts.KindExtension
)

// KindOrder is the display order of the kind groups, most-common first.
var KindOrder = facts.KindOrder

// Catalog returns every construct this engine can discover. That covers every fact-base relation,
// every derived relation defined by a library module, and every predicate in the fact layer's
// vocabulary, including the engine's own string tests. The result is sorted by kind (KindOrder)
// then name, so a caller renders a stable grouped list without re-sorting.
func Catalog() []RelationInfo { return CatalogFrom(facts.DefaultRegistry()) }

// CatalogFrom is Catalog over an explicit relation vocabulary.
func CatalogFrom(reg *facts.Registry) []RelationInfo {
	rels := reg.Relations()
	preds, derived := reg.Predicates(), reg.Derived()
	out := make([]RelationInfo, 0, len(rels)+len(preds)+len(derived))
	out = append(out, rels...)
	out = append(out, derived...)
	out = append(out, preds...)
	// A predicate's reference markdown resolves through the doc registry, as a relation's does. So
	// a documented predicate lists with its Detail, and an undocumented one still lists with its
	// Summary.
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

// Entry describes one path of the vocabulary. It holds either a module and its members, or a member
// with its signature, kind and doc, plus its definition for a derived relation. See ns.Entry.
type Entry = ns.Entry

// Describe answers what is at path in the process-default vocabulary, "" being the root. An unknown
// path returns an error worded as a query would word it, with a suggestion. If a library's modules
// do not check, Describe returns that error rather than a partial answer.
func Describe(path string) (Entry, error) { return DescribeFrom(facts.DefaultRegistry(), path) }

// DescribeFrom is Describe over an explicit vocabulary.
func DescribeFrom(reg *facts.Registry, path string) (Entry, error) {
	return reg.Vocabulary().Lookup(path)
}

// ModuleMembers reports the public members defined in a Datalog module's text, by bare name. A host
// uses it to check a module's paths against a vocabulary before composing it. It returns an error
// when the module does not parse, or when the language refuses it on its own terms.
func ModuleMembers(text string) ([]string, error) {
	decls, err := datalog.Language.Members(text)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(decls))
	for i, d := range decls {
		out[i] = d.Name
	}
	return out, nil
}
