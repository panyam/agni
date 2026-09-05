// Package facts is the fact/relation layer: the tuple a relation projects a check.Model into, and
// the registry those relations install themselves in. It is the primitive the query surfaces sit on
// and it depends on no query engine (C29), so a relation is authored once and every engine that
// wants to answer questions over the design reads the same rows.
//
// The split it enforces: a RELATION is data derived from the Model (this package), while a
// PREDICATE, a join strategy, and a query language are an engine's own business (core/query holds
// the datalog one). An overlay contributing house facts imports this package and never a query
// engine; an engine imports this package to read what has been registered.
package facts

// Row is one tuple of the derived fact base: the shape a projector produces and an engine indexes,
// joins, and returns provenance from.
//
// NOT an answer type. What a query returns is the engine's own shape, carrying as many columns as
// the query projects. The fixed field set below bounds what one FACT can say, never what a query can
// return, which is why a relation wider than these slots is a change here rather than in any
// evaluator.
//
// Subject is the primary entity (a net name, ref-des, or mpn); Object is the second entity or
// attribute key (the net for component-on-net, the symbol for param, "" otherwise). Value is the
// rendered value ("" for a pure link like component-on-net); Num carries it as a number when the
// relation is numeric, so a consumer can range or compare without re-parsing. Min carries a SECOND
// numeric slot — the lower bound of a two-sided range relation (param.range), where Num is the upper
// bound; it is nil for every one-number relation. Conditions holds a param's test conditions ("" otherwise).
// Qualifier is the SECOND STRING slot, standing to Value exactly as Min stands to Num: it exists for
// a relation that must discriminate rows on a dimension Value is already spent on (param.pin_range
// carries the symbol in Value and the limit kind here, because collapsing an absolute maximum and a
// recommended range onto one row would be the exact confusion LimitKind exists to prevent). Empty
// for every relation that does not need it.
//
// Cites are the rendered provenance of the fact: an IR source, or a datasheet doc/page/table. NEVER
// EMPTY for a well-formed fact, because a fact you cannot cite is not verifiable, and an empty slice
// is a new way to say nothing that an empty string would have shown (TestEveryFactCitesSomething is
// what holds it).
//
// A SLICE because a fact can rest on several sites, and the plurality is sometimes the whole point.
// A ref-des collision IS several placements sharing one designator, so citing one of them withholds
// exactly what a reviewer is chasing; a part stating both an air-discharge and a contact-discharge
// ESD rating earns its credit from either (agni issue 546).
//
// NOT a bindable slot. Nothing in the relation schema names it, query.fieldValue never returns it,
// and the evaluator only accumulates it onto an answer, so widening it reaches neither the
// binding-pattern index nor the join. That is why this could grow while the tuple's ARITY stayed
// fixed.
// BaseUnit is the SI BASE symbol Num and Min are expressed in ("V", "A", "Ω"), or "" when the
// relation is dimensionless or non-numeric. Both numeric slots share it, since a two-sided range's
// bounds are always the same dimension. It must never be a prefixed spelling; see Value.BaseUnit.
type Row struct {
	Relation   string
	Subject    string
	Object     string
	Value      string
	Num        *float64
	Min        *float64
	BaseUnit   string
	Conditions string
	Qualifier  string
	Cites      []string
}
