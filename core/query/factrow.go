package query

// FactRow is one tuple of the derived fact base — the row shape the datalog engine indexes,
// joins, and returns provenance from. It lives here (the query layer) because it is the engine's
// INPUT tuple: a projector in stdlib/relations produces []FactRow, NewBase indexes them by
// Relation, and the evaluator binds a query's positional terms to a row's fields via the edbField
// layout (schema.go). The relations that FILL it are stdlib content (WS10 / issue 10); this is the
// tuple they fill.
//
// NOT the answer type. A query's results are []Row (query.go), whose Bind is a map and therefore
// carries as many variables as the query projects. The fixed field set below bounds what one FACT
// can say, never what a query can return, which is why adding a relation wider than these slots is
// a change here rather than anywhere in the evaluator.
//
// Subject is the primary entity (a net name, ref-des, or mpn); Object is the second entity or
// attribute key (the net for component-on-net, the symbol for param, "" otherwise). Value is the
// rendered value ("" for a pure link like component-on-net); Num carries it as a number when the
// relation is numeric, so a consumer can range or compare without re-parsing. Min carries a SECOND
// numeric slot — the lower bound of a two-sided range relation (param.range), where Num is the upper
// bound; it is nil for every one-number relation. Conditions holds a param's test conditions (""
// otherwise). Qualifier is the SECOND STRING slot, standing to Value exactly as Min stands to Num:
// it exists for a relation that must discriminate rows on a dimension Value is already spent on
// (param.pin_range carries the symbol in Value and the limit kind here, because collapsing an
// absolute maximum and a recommended range onto one row would be the exact confusion LimitKind
// exists to prevent). Empty for every relation that does not need it. Cite is the rendered provenance — an IR source or a datasheet doc/page/table — and is
// never empty for a well-formed fact: a fact you cannot cite is not verifiable.
// BaseUnit is the SI BASE symbol Num and Min are expressed in ("V", "A", "\u03a9"), or "" when the
// relation is dimensionless or non-numeric. Both numeric slots share it, since a two-sided range's
// bounds are always the same dimension. It must never be a prefixed spelling; see Value.BaseUnit.
type FactRow struct {
	Relation   string
	Subject    string
	Object     string
	Value      string
	Num        *float64
	Min        *float64
	BaseUnit   string
	Conditions string
	Qualifier  string
	Cite       string
}
