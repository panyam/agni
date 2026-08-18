package query

// The find-by-name template: the query the viewer runs when someone TYPES a name instead of
// clicking a thing on the drawing.
//
// It lives here for the reason the entity presets and the examples do. It names `entity` and
// `match`, both defined in Go, so a copy held in the browser would be the one caller nothing
// checks. Beside them it inherits the parse check in this package and the evaluate-against-a-real-
// design check at the RPC layer.
//
// The choice of `entity` is the whole point of that relation existing. Every other relation ranges
// over an association, so a name search built on one silently cannot find a part with no
// connections or a net with nothing on it, which are exactly the things a reviewer is hunting.
//
// `match` rather than `contains` for two reasons. A search box that only matches case is a search
// box a newcomer gives up on, and `(?i)` is how you say otherwise. And a reader who wants "every
// designator starting with U" can write `^U` in the box they are already typing in, which is the
// same bargain the rest of the panel makes: answer the question, and leave the language behind.
//
// The caller substitutes {term} with the reader's text, regex-escaped (see the web client's
// searchPattern). Escaping is the caller's job because only the caller knows whether the text came
// from a human typing a name or from something that already is a pattern.

// SearchQuery is the find-by-name template, with Teaches carrying the concept a search leaves
// behind the way an example's does.
type SearchQuery struct {
	Query   string
	Teaches string
}

// Search returns the find-by-name template. One template rather than one per kind: the answer names
// the kind of every hit in its own column, so a reader searching "CAN" sees the net, the connector
// and the bus label together and learns that those are three different sorts of thing.
func Search() SearchQuery {
	return SearchQuery{
		Query:   `entity(?name, ?kind), match(?name, "(?i){term}")`,
		Teaches: "entity(?name, ?kind) enumerates what a design NAMES, so a search finds the parts and nets that no connection reaches",
	}
}
