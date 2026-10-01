package query

// Entity presets: the query a viewer runs when someone clicks a thing in the drawing.
//
// These live in Go rather than in the browser because every one names relations (pin.net,
// component-on-net, net.pin_count) defined in Go, and a client-side template would break at runtime
// when a relation is renamed with no test going red. Here they share the examples' two guards, the
// parse check in this package and the evaluate-against-a-real-design check at the RPC layer.
//
// Each preset names what the thing is attached to and stops. A click starts a question the reader
// then edits, which is how they learn the query language.

// EntityQuery is the preset for one entity kind. Query carries placeholders the caller substitutes:
// {ref}, {pin}, {net}, {bus}. They sit INSIDE the quotes in the template ("{ref}"), so a template
// parses as-is and the parse test needs no substitution pass to be meaningful.
type EntityQuery struct {
	Kind    string // "pin" | "component" | "net" | "bus", matching a picked selection's kind
	Query   string
	Teaches string // the concept this preset introduces, shown the way an example's is
}

// EntityQueries returns one preset per entity kind a viewer can pick.
func EntityQueries() []EntityQuery {
	return []EntityQuery{
		{
			Kind: "pin",
			// The pin question is "is this wired correctly", which starts with what the pin is attached
			// to, what the pin is FOR, and how many other things share that net. A power pin on a net with a fan-out of
			// one is the shape of a real defect.
			Query:   `pin.net("{ref}", "{pin}", ?net), pin.role("{ref}", "{pin}", ?role), net.pin_count(?net, ?fanout) => ?net, ?role, ?fanout`,
			Teaches: "join: one pin's net, its role, and that net's fan-out come from three relations sharing ?net",
		},
		{
			Kind:    "component",
			Query:   `component-on-net("{ref}", ?net), net.pin_count(?net, ?fanout) => ?net, ?fanout`,
			Teaches: "projection: => picks which columns the answer keeps",
		},
		{
			Kind: "net",
			// component-on-net answers WHO is on the net and pin.net answers through which terminal;
			// the join turns a list of parts into a wiring list.
			Query:   `component-on-net(?ref, "{net}"), pin.net(?ref, ?pin, "{net}") => ?ref, ?pin`,
			Teaches: "join: a shared ?ref connects the parts on a net to the pins that land on it",
		},
		{
			Kind:    "bus",
			Query:   `bus("{bus}", ?member) => ?member`,
			Teaches: "a bus is a relation over its members, not a net",
		},
	}
}
