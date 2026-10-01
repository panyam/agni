package query

// Entity presets: the query a viewer runs when someone clicks a thing in the drawing.
//
// These live in Go rather than in the browser because every one names relations (pin.net,
// component.net, net.pin_count) defined in Go, and a client-side template would break at runtime
// when a relation is renamed with no test going red. Here they share the examples' two guards, the
// parse check in this package and the evaluate-against-a-real-design check at the RPC layer.
//
// Each preset names what the thing is attached to and stops. A click starts a question the reader
// then edits, which is how they learn the query language.

// EntityQuery is the preset for one entity kind. The caller BINDS the variables Binds names from
// the picked selection (agni issue 793) rather than splicing values into the text, so the query is
// the same for every click and a designator holding a quote is asked about exactly. Each name is
// also the selection field that fills it: ref, pin, net, or bus.
//
// Binds is explicit rather than every field a selection carries, because one preset's input is
// another's output. The pin preset answers ?net and the net preset is asked about one.
type EntityQuery struct {
	Kind    string // "pin" | "component" | "net" | "bus", matching a picked selection's kind
	Query   string
	Binds   []string
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
			Query:   `pin.net(?ref, ?pin, ?net), pin.role(?ref, ?pin, ?role), net.pin_count(?net, ?fanout) => ?net, ?role, ?fanout`,
			Binds:   []string{"ref", "pin"},
			Teaches: "join: one pin's net, its role, and that net's fan-out come from three relations sharing ?net",
		},
		{
			Kind:    "component",
			Query:   `component.net(?ref, ?net), net.pin_count(?net, ?fanout) => ?net, ?fanout`,
			Binds:   []string{"ref"},
			Teaches: "projection: => picks which columns the answer keeps",
		},
		{
			Kind: "net",
			// component.net answers WHO is on the net and pin.net answers through which terminal;
			// the join turns a list of parts into a wiring list.
			Query:   `component.net(?ref, ?net), pin.net(?ref, ?pin, ?net) => ?ref, ?pin`,
			Binds:   []string{"net"},
			Teaches: "join: a shared ?ref connects the parts on a net to the pins that land on it",
		},
		{
			Kind:    "bus",
			Query:   `bus(?bus, ?member) => ?member`,
			Binds:   []string{"bus"},
			Teaches: "a bus is a relation over its members, not a net",
		},
	}
}
