package query

// FactRow is one tuple of the derived fact base — the row shape the datalog engine indexes,
// joins, and returns provenance from. It lives here (the query layer) because it is the engine's
// result/cursor type: a projector in stdlib/relations produces []FactRow, NewBase indexes them by
// Relation, and the evaluator binds a query's positional terms to a row's fields via the edbField
// layout (schema.go). The relations that FILL it are stdlib content (WS10 / issue 10); this is the
// tuple they fill.
//
// Subject is the primary entity (a net name, ref-des, or mpn); Object is the second entity or
// attribute key (the net for component-on-net, the symbol for param, "" otherwise). Value is the
// rendered value ("" for a pure link like component-on-net); Num carries it as a number when the
// relation is numeric, so a consumer can range or compare without re-parsing. Min carries a SECOND
// numeric slot — the lower bound of a two-sided range relation (param.range), where Num is the upper
// bound; it is nil for every one-number relation. Conditions holds a param's test conditions (""
// otherwise). Cite is the rendered provenance — an IR source or a datasheet doc/page/table — and is
// never empty for a well-formed fact: a fact you cannot cite is not verifiable.
type FactRow struct {
	Relation   string
	Subject    string
	Object     string
	Value      string
	Num        *float64
	Min        *float64
	Conditions string
	Cite       string
}
