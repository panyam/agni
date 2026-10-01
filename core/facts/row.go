// Package facts is the fact/relation layer. It holds the tuple a relation projects a check.Model
// into, and the registry those relations install themselves in. It depends on no query engine (C29), so a
// relation is authored once and every engine that answers questions over the design reads the same
// rows.
//
// A RELATION is data derived from the Model and lives here. A PREDICATE, a join strategy and a query
// language are an engine's own business (core/query holds the datalog one). An extension contributing
// house facts imports this package and never a query engine.
package facts

// Row is one tuple of the derived fact base, produced by a projector and indexed, joined and cited by
// an engine.
//
// NOT an answer type. A query returns the engine's own shape, with as many columns as it projects.
// These fields bound what one FACT can say, so a relation wider than these slots is a change here
// rather than in any evaluator.
//
// Subject is the primary entity (a net name, ref-des, or mpn). Object is the second entity or
// attribute key (the net for component.net, the symbol for param, "" otherwise). Value is the
// rendered value ("" for a pure link like component.net), and Num carries it as a number when the
// relation is numeric, so a consumer can range or compare without re-parsing. Min is a SECOND numeric
// slot, the lower bound of a two-sided range relation (param.range) where Num is the upper bound, and
// nil for every one-number relation. Conditions holds a param's test conditions ("" otherwise).
//
// Qualifier is the SECOND STRING slot, standing to Value as Min stands to Num, for a relation that
// must discriminate rows on a dimension Value is already spent on. param.pin_range carries the symbol
// in Value and the limit kind here, so an absolute maximum and a recommended range never collapse onto
// one row. Empty for every relation that does not need it.
//
// BaseUnit is the SI BASE symbol Num and Min are expressed in ("V", "A", "Ω"), or "" when the
// relation is dimensionless or non-numeric. Both numeric slots share it. It must never be a prefixed
// spelling; see query.Value.BaseUnit.
//
// Cites are the rendered provenance of the fact, an IR source or a datasheet doc/page/table. NEVER
// EMPTY for a well-formed fact, since a fact you cannot cite is not verifiable
// (TestEveryFactCitesSomething holds it). A slice because a fact can rest on several sites. A ref-des
// collision IS several placements sharing one designator, and a part stating both an air-discharge
// and a contact-discharge ESD rating earns its credit from either (agni issue 546). Cites is not a
// bindable slot, since nothing in the relation schema names it and query.fieldValue never returns it,
// so it grew without changing the tuple's ARITY.
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
