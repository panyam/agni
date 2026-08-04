package query

import (
	"fmt"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// This is the built-in analogue of RegisterRelation, the symmetric twin of check.RegisterBuiltins
// for the rule side (issue 10). The query engine owns no relations: the standard EDB relations —
// netlist, board, and datasheet projections over a Model — are first-party CONTENT that
// stdlib/relations registers here at init, exactly as stdlib/rules/builtin registers the standard
// rule catalog with check. A binary that omits the stdlib/relations import runs the query engine
// with no built-in relations (only overlay-registered ones and the computed predicates), the same
// posture as a binary that omits the rule catalog.
//
// Bulk, not per-relation: RegisterRelation stays the per-relation OVERLAY seam (one relation at a
// time, namespaced), while the built-ins arrive as one payload because their projector is a single
// monolithic pass (relations.Facts) and several share it — forcing them through per-relation
// registration would fight that shape for no gain (the rule side made the same call, issue 4 phase
// 2b). The seam lives in query, not check, so check stays unaware of the query layer.

// SpecLibProjector derives fact rows from a whole datasheet spec library with NO design (WS10-010),
// the library-wide analogue of Projector. NewSpecLibBase iterates it so `agni query --speclib`
// searches the corpus; a nil projector (no stdlib/relations imported) yields an empty library base.
type SpecLibProjector func([]*parampb.PartSpec) []FactRow

// BuiltinFacts is the bulk payload stdlib/relations installs. Schema is each relation's positional
// field layout (merged into the engine's schema so the evaluator, negation, and arity checks treat a
// built-in relation exactly like an overlay one); Catalog is the human-facing picker metadata for
// those relations; Model and SpecLib are the design-scoped and library-wide projectors; Doc resolves
// a relation's reference markdown ("" when it has none). Everything here is derived from a Model or a
// spec library — never a second authoritative store (C8).
type BuiltinFacts struct {
	Schema  map[string][]Field
	Catalog []RelationInfo
	Model   Projector
	SpecLib SpecLibProjector
	Doc     func(name string) string
}

var (
	builtinFactsModel      Projector
	builtinFactsSpecLib    SpecLibProjector
	builtinRelationCatalog []RelationInfo
	builtinDocFn           func(name string) string
)

// RegisterBuiltinFacts installs the standard EDB relations as query content. stdlib/relations calls
// it from its init, so any binary that blank-imports that package has the built-in relations
// available before it builds a Base. It merges each relation's layout into the engine schema and
// stores the two projectors, the picker catalog, and the doc resolver. It panics on a name that
// collides with an already-registered built-in relation or predicate — a duplicate registration is a
// programming error that must fail loudly at load, the same contract as RegisterRelation.
func RegisterBuiltinFacts(bf BuiltinFacts) {
	for name, fields := range bf.Schema {
		if _, ok := edbSchema[name]; ok {
			panic(fmt.Sprintf("query: RegisterBuiltinFacts(%q) collides with a registered built-in relation", name))
		}
		if _, ok := builtins[name]; ok {
			panic(fmt.Sprintf("query: RegisterBuiltinFacts(%q) collides with a built-in predicate", name))
		}
		ef := make([]edbField, len(fields))
		for i, f := range fields {
			ef[i] = edbField(f)
		}
		edbSchema[name] = ef
	}
	builtinFactsModel = bf.Model
	builtinFactsSpecLib = bf.SpecLib
	builtinRelationCatalog = bf.Catalog
	builtinDocFn = bf.Doc
}

// relationDoc resolves a relation's reference markdown through the registered doc resolver, or "" if
// stdlib/relations is not imported or the relation has no doc. Catalog() uses it so query need not
// import the relations package (which would cycle: relations imports query).
func relationDoc(name string) string {
	if builtinDocFn == nil {
		return ""
	}
	return builtinDocFn(name)
}
