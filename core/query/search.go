package query

// The find-by-name template is the query the viewer runs when someone TYPES a name instead of
// clicking a thing on the drawing.
//
// It lives in Go beside the entity presets and the examples, because it names `entity` and
// `str.match`, both defined in Go, so a copy held in the browser would go unchecked. Here it gets
// this package's parse check and the RPC layer's evaluate-against-a-real-design check.
//
// It uses `entity` because every other relation ranges over an association, so a search built on
// one cannot find a part with no connections or a net with nothing on it
// (docsite/content/guide/querying.md#find-something-by-name). It uses `str.match` rather than
// `str.contains` so the search is case-insensitive via `(?i)` and a reader can type a pattern such
// as `^U`.
//
// The caller BINDS ?pattern rather than splicing text into the query (agni issue 793), so the query
// is the same text for every search. The bound value is Pattern with {term} replaced by the reader's
// text, regex-escaped (see the web client's searchPattern), since the value is a regex. Escaping is
// the caller's job because only the caller knows whether the text is a typed name or already a
// pattern.

// SearchQuery is the find-by-name template, with Teaches carrying the concept a search leaves
// behind the way an example's does.
type SearchQuery struct {
	Query string
	// Bind names the variable the caller binds, without its `?`.
	Bind string
	// Pattern is the value bound to Bind, with {term} standing for the escaped text. It holds the
	// (?i) here rather than in each client, so the search is case-insensitive wherever it runs.
	Pattern string
	Teaches string
}

// Search returns the find-by-name template. It is one template rather than one per kind, since the
// answer names each hit's kind in its own column, so a search for "CAN" shows the net, the connector
// and the bus label together.
func Search() SearchQuery {
	return SearchQuery{
		Query:   `entity(?name, ?kind), str.match(?name, ?pattern) => ?name, ?kind`,
		Bind:    "pattern",
		Pattern: "(?i){term}",
		Teaches: "entity(?name, ?kind) enumerates what a design NAMES, so a search finds the parts and nets that no connection reaches",
	}
}
