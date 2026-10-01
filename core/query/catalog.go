package query

import (
	"sort"

	"github.com/panyam/agni/core/facts"
	"github.com/panyam/jaala/datalog"
	"github.com/panyam/jaala/ns"
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
	KindDerived   = facts.KindDerived
	KindExtension = facts.KindExtension
)

// KindOrder is the display order of the kind groups, most-common first.
var KindOrder = facts.KindOrder

// Catalog returns this engine's discoverable construct set: every fact-base relation, every derived
// relation a library module defines, and every predicate in the fact layer's vocabulary, the
// engine's own string tests included. The result is
// sorted by kind (KindOrder) then name, so a caller renders a stable grouped list without re-sorting.
func Catalog() []RelationInfo { return CatalogFrom(facts.DefaultRegistry()) }

// CatalogFrom is Catalog over an explicit relation vocabulary.
func CatalogFrom(reg *facts.Registry) []RelationInfo {
	rels := reg.Relations()
	preds, derived := reg.Predicates(), reg.Derived()
	out := make([]RelationInfo, 0, len(rels)+len(preds)+len(derived))
	out = append(out, rels...)
	out = append(out, derived...)
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

// Entry is what sits at one path of the vocabulary: a module and its members, or a member with its
// signature, kind, doc and, for a derived relation, its definition. See ns.Entry.
type Entry = ns.Entry

// Describe answers what is at path in the process-default vocabulary, "" being the root. An unknown
// path is an error worded as a query would word it, suggestion included, and a library whose
// modules do not check returns that error rather than a partial answer.
func Describe(path string) (Entry, error) { return DescribeFrom(facts.DefaultRegistry(), path) }

// DescribeFrom is Describe over an explicit vocabulary.
func DescribeFrom(reg *facts.Registry, path string) (Entry, error) {
	return reg.Vocabulary().Lookup(path)
}

// ModuleMembers reports the public members a Datalog module's text defines, by bare name, so a host
// can check a module's paths against a vocabulary before composing it. An error is a module that does
// not parse, or that the language refuses on its own terms.
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
