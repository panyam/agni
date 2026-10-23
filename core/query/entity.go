package query

// Entity presets: the questions a viewer offers about a thing the reader selected in the drawing.
//
// These live in Go rather than in the browser because every one names relations (pin.net,
// component.net, net.pin_count) defined in Go, and a client-side template would break at runtime
// when a relation is renamed with no test going red. Here they share the examples' two guards, the
// parse check in this package and the evaluate-against-a-real-design check at the RPC layer.
//
// A kind has several questions, and the viewer lists them as the Properties panel's Ask rows (agni
// issue 902). Each opens its own query tab, so the reader edits the answer's query rather than a
// box someone else's click overwrites, which is how they learn the query language.

// EntityQuery is one question about one entity kind. The caller BINDS the variables Binds names
// from the selection (agni issue 793) rather than splicing values into the text, so the query is
// the same for every selection and a designator holding a quote is asked about exactly. Each name
// is also the selection field that fills it: ref, pin, net, or bus.
//
// Binds is explicit rather than every field a selection carries, because one preset's input is
// another's output. The pin preset answers ?net and the net preset is asked about one.
type EntityQuery struct {
	Kind    string // "pin" | "component" | "net" | "bus" | "area", matching a picked selection's kind
	Label   string // the question as the Ask row words it, unique within its kind
	Query   string
	Binds   []string
	Teaches string // the concept this preset introduces, shown the way an example's is
}

// EntityQueries returns the questions for every entity kind a viewer can select, grouped by kind.
// The first question of a kind is its default, the one a viewer asks when it offers only one.
func EntityQueries() []EntityQuery {
	return []EntityQuery{
		{
			Kind:  "pin",
			Label: "What is it wired to?",
			// The pin question is "is this wired correctly", which starts with what the pin is attached
			// to, what the pin is FOR, and how many other things share that net. A power pin on a net with a fan-out of
			// one is the shape of a real defect.
			Query:   `pin.net(?ref, ?pin, ?net), pin.role(?ref, ?pin, ?role), net.pin_count(?net, ?fanout) => ?net, ?role, ?fanout`,
			Binds:   []string{"ref", "pin"},
			Teaches: "join: one pin's net, its role, and that net's fan-out come from three relations sharing ?net",
		},
		{
			Kind:    "pin",
			Label:   "What else is on its net?",
			Query:   `pin.net(?ref, ?pin, ?net), pin.net(?other, ?other_pin, ?net), ?other != ?ref => ?other, ?other_pin, ?net`,
			Binds:   []string{"ref", "pin"},
			Teaches: "a self-join: the same relation twice, joined on ?net, with a comparison dropping the pin's own part",
		},
		{
			Kind:    "component",
			Label:   "What nets does it touch?",
			Query:   `component.net(?ref, ?net), net.pin_count(?net, ?fanout) => ?net, ?fanout`,
			Binds:   []string{"ref"},
			Teaches: "projection: => picks which columns the answer keeps",
		},
		{
			Kind:    "component",
			Label:   "What does each pin do?",
			Query:   `pin.net(?ref, ?pin, ?net), pin.role(?ref, ?pin, ?role) => ?pin, ?role, ?net`,
			Binds:   []string{"ref"},
			Teaches: "two relations keyed on the same part and pin, read side by side",
		},
		{
			Kind:    "component",
			Label:   "Which test points reach its nets?",
			Query:   `component.net(?ref, ?net), component.net(?tp, ?net), component.class(?tp, "test_point") => ?net, ?tp`,
			Binds:   []string{"ref"},
			Teaches: "a constant in a goal: component.class(?tp, \"test_point\") keeps only the test points",
		},
		{
			Kind:    "component",
			Label:   "What does its datasheet say?",
			Query:   `component.mpn(?ref, ?mpn), param.typ(?mpn, ?param, ?typ) => ?mpn, ?param, ?typ`,
			Binds:   []string{"ref"},
			Teaches: "the datasheet tier: param relations join a part through its part number, and answer nothing without a corpus",
		},
		{
			Kind: "net",
			// pin.net already names the part and the terminal, so this is one indexed lookup on the
			// bound net. It used to join component.net first, which added no row and cost one probe
			// per row: 140 ms against 6 ms for the 1229 pins on a large board's GND (agni issue 896).
			Label:   "What pins are on it?",
			Query:   `pin.net(?ref, ?pin, ?net) => ?ref, ?pin`,
			Binds:   []string{"net"},
			Teaches: "a bound argument: with ?net given, pin.net answers only that net's pins",
		},
		{
			Kind:    "net",
			Label:   "Which test points are on it?",
			Query:   `component.net(?tp, ?net), component.class(?tp, "test_point") => ?tp`,
			Binds:   []string{"net"},
			Teaches: "a constant in a goal: component.class(?tp, \"test_point\") keeps only the test points",
		},
		{
			Kind:    "net",
			Label:   "Where does it reach through series parts?",
			Query:   `net.route(?net, ?to, ?path) => ?to, ?path`,
			Binds:   []string{"net"},
			Teaches: "a generator: net.route walks through resistors and ferrites from a bound net and names the path it took",
		},
		{
			// bus records the construct a reader saw, not its members, so this is the one question
			// about a bus the relations can answer. Its members are on the IR, which Properties reads.
			Kind:    "bus",
			Label:   "What construct is it drawn as?",
			Query:   `bus(?bus, ?construct) => ?construct`,
			Binds:   []string{"bus"},
			Teaches: "a bus is recorded as the construct the reader saw (bus_alias, edif_array), not yet as its member nets",
		},
		// An AREA is a set of parts, pins and wires selected together on one sheet (agni issue 904).
		// Its questions bind ?ref to every member part and ?net to every net in the area at once, so a
		// question is asked once and an aggregate reduces across the whole set.
		{
			Kind: "area",
			// A boundary net is one the area's parts reach but do not hold every pin of, which is the
			// block's interface. Each binding is one member pin on the net, so count(?pin) is how many
			// of the net's pins are inside.
			Label:   "What does this block connect to?",
			Query:   `pin.net(?ref, ?pin, ?net), net.pin_count(?net, ?pins) => ?net, ?pins, count(?pin) having count(?pin) < ?pins`,
			Binds:   []string{"ref"},
			Teaches: "a set binding and having: ?ref holds every part in the area, and having keeps the nets whose pins are not all inside",
		},
		{
			Kind:    "area",
			Label:   "Which of its nets have no test point?",
			Query:   `net.pin_count(?net, ?pins), not net.has_test_point(?net) => ?net, ?pins`,
			Binds:   []string{"net"},
			Teaches: "negation over a library member: not net.has_test_point keeps the nets nothing probes",
		},
		{
			Kind:  "area",
			Label: "What parts are in it, by class?",
			// ?part is what the aggregates reduce, filtered to the area, since an area of one part binds
			// ?ref as a constant and a constant cannot be counted.
			Query:   `component.class(?part, ?class), ?part = ?ref => ?class, count(?part), list(?part)`,
			Binds:   []string{"ref"},
			Teaches: "aggregation across a bound set: count and list reduce over every part the area holds",
		},
	}
}
