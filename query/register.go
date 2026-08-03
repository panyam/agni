package query

import (
	"fmt"

	"github.com/panyam/agni/check"
)

// This is the overlay seam (WS3-029 fast-follow, the open-core story generalized to the query
// surface): the public engine ships the built-in EDB relations (netlist, datasheet, board), and a
// private overlay contributes its OWN relations — house part attributes, a compliance database, an
// approved-vendor feed — without editing the engine. A registered relation is a first-class query
// citizen: the goal joins it, rules read it, negation ranges over it, all with no evaluator change,
// because the evaluator already treats every EDB relation uniformly (name -> field layout -> facts).
//
// It mirrors the check.Facts discipline: a relation is a name, a positional layout over FactRow, and
// a Projector that DERIVES its rows from a Model (never a second authoritative store, C8). The seam
// lives here rather than in check so check stays unaware of the query layer; an overlay imports both.

// Field names the check.FactRow field a registered relation's positional argument binds to. The
// values mirror the internal field enum, so a registration reads as, e.g.,
// []Field{FieldSubject, FieldNum} for reln(subject, number).
type Field int

const (
	FieldSubject    = Field(fSubject)    // FactRow.Subject — the primary entity (net, ref-des, mpn)
	FieldObject     = Field(fObject)     // FactRow.Object — the second entity or attribute key
	FieldValue      = Field(fValue)      // FactRow.Value — the rendered string value
	FieldNum        = Field(fNum)        // FactRow.Num — the numeric value (for range/compare)
	FieldConditions = Field(fConditions) // FactRow.Conditions — a parameter's test conditions
)

// Projector derives a relation's fact rows from a Model, the same shape as check's per-relation
// projectors. It runs once per NewBase; an empty result is correct when the Model lacks the tier the
// relation needs (silent-by-construction, never fabricated), matching the built-in relations.
type Projector func(check.Model) []check.FactRow

type relationDef struct {
	fields  []edbField
	project Projector
}

var (
	registry      = map[string]relationDef{}
	registryOrder []string // registration order, so NewBase merges deterministically
)

// RegisterRelation adds an overlay-supplied EDB relation to the query surface. name is the relation
// as queries write it (e.g. "house.approved"), fields is its positional layout over FactRow, and
// project derives its rows from a Model. Call it once at init time. It panics on a misregistration —
// an empty name or field list, or a name that collides with a built-in relation, a built-in
// predicate (reaches, the string filters), or an already-registered relation — because a duplicate
// or shadowing relation is a programming error that must fail loudly at load, not silently at query
// time (the same contract as net/http.Handle).
func RegisterRelation(name string, fields []Field, project Projector) {
	if name == "" {
		panic("query: RegisterRelation with empty name")
	}
	if len(fields) == 0 {
		panic(fmt.Sprintf("query: RegisterRelation(%q) with no fields", name))
	}
	if project == nil {
		panic(fmt.Sprintf("query: RegisterRelation(%q) with nil projector", name))
	}
	if _, ok := edbSchema[name]; ok {
		panic(fmt.Sprintf("query: RegisterRelation(%q) collides with a built-in relation", name))
	}
	if _, ok := builtins[name]; ok {
		panic(fmt.Sprintf("query: RegisterRelation(%q) collides with a built-in predicate", name))
	}
	if _, ok := registry[name]; ok {
		panic(fmt.Sprintf("query: RegisterRelation(%q) already registered", name))
	}
	ef := make([]edbField, len(fields))
	for i, f := range fields {
		ef[i] = edbField(f)
	}
	registry[name] = relationDef{fields: ef, project: project}
	registryOrder = append(registryOrder, name)
}

// RegisterPredicate adds an overlay-supplied filter predicate to the query surface. name is how
// queries call it, arity its argument count, and holds the boolean it computes over the (all-bound)
// argument values. It is a pure filter, the same kind as the built-in contains/prefix/suffix: it
// keeps a binding when holds is true, and `not name(...)` keeps it when holds is false — both derived
// from the one function, so they can never disagree. holds must depend only on its arguments: no
// Model or fact-base access. That is deliberate. A predicate that could ENUMERATE bindings from the
// Model (a generator, like reaches) can produce values, and a value-producing generator would break
// the finiteness guarantee that makes evaluation terminate — so a generator seam is withheld until a
// real need justifies designing it safely. Panics on the same misregistrations as RegisterRelation.
func RegisterPredicate(name string, arity int, holds func(args []Value) (bool, error)) {
	if name == "" {
		panic("query: RegisterPredicate with empty name")
	}
	if arity < 1 {
		panic(fmt.Sprintf("query: RegisterPredicate(%q) needs arity >= 1", name))
	}
	if holds == nil {
		panic(fmt.Sprintf("query: RegisterPredicate(%q) with nil predicate", name))
	}
	if _, ok := edbSchema[name]; ok {
		panic(fmt.Sprintf("query: RegisterPredicate(%q) collides with a built-in relation", name))
	}
	if _, ok := builtins[name]; ok {
		panic(fmt.Sprintf("query: RegisterPredicate(%q) collides with a built-in predicate", name))
	}
	if _, ok := registry[name]; ok {
		panic(fmt.Sprintf("query: RegisterPredicate(%q) collides with a registered relation", name))
	}
	builtins[name] = filterBuiltin(arity, holds)
}

// edbSchemaOf resolves an EDB relation's positional layout from the built-in schema first, then the
// overlay registry — so the evaluator, negation, and arity checks treat a registered relation
// exactly like a built-in one.
func edbSchemaOf(rel string) ([]edbField, bool) {
	if fields, ok := edbSchema[rel]; ok {
		return fields, true
	}
	if d, ok := registry[rel]; ok {
		return d.fields, true
	}
	return nil, false
}

// isEDBRelation reports whether a relation is a fact-base relation (built-in or overlay-registered),
// so a rule may not redefine it and a rule body may read it.
func isEDBRelation(rel string) bool {
	_, ok := edbSchemaOf(rel)
	return ok
}
