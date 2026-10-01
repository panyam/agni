package query

// ExampleQuery is one runnable teaching query (WS14-002): a plain-language intent, the datalog text
// that answers it, and the one concept it introduces. It is the shared catalog behind BOTH the web
// panel's click-to-run examples and the CLI's `agni query --examples`, so the two surfaces cannot
// drift.
type ExampleQuery struct {
	Label   string // plain-language intent, shown on the chip ("Parts on a rail above 3V")
	Query   string // the datalog text the chip fills and runs
	Teaches string // the one concept this rung introduces ("join", "filter", ...)
}

// examples is the concept ladder, in teaching order: each rung adds exactly one idea over the last
// (projection → filter → join → predicate → recursion). The set is design-independent, but ordered
// so the first (component.net) returns rows on any netlist while later rungs may return none on a
// design that lacks the data, and that empty answer is itself a lesson (WS14-001).
var examples = []ExampleQuery{
	{
		Label:   "Every part on every net",
		Query:   "component.net(?ref, ?net) => ?ref, ?net",
		Teaches: "projection: => picks the answer columns",
	},
	{
		Label:   "Rails above 3V",
		Query:   "net.max_voltage(?net, ?v), ?v > 3 => ?net, ?v",
		Teaches: "filter: a bare comparison prunes rows",
	},
	{
		Label:   "Parts sitting on a rail above 3V",
		Query:   "component.net(?ref, ?net), net.max_voltage(?net, ?v), ?v > 3 => ?ref, ?net, ?v",
		Teaches: "join: a shared ?variable connects two relations",
	},
	{
		Label:   "Parts on USB nets",
		Query:   `component.net(?ref, ?net), str.contains(?net, "USB") => ?ref, ?net`,
		Teaches: "predicate: a string test over a bound value",
	},
	{
		Label:   "Anything named like this",
		Query:   `entity(?name, ?kind), str.contains(?name, "USB") => ?name, ?kind`,
		Teaches: "enumeration: entity ranges over what EXISTS, so a name search also finds what nothing is joined to",
	},
	{
		Label:   "Reachable through series pass elements",
		Query:   "net.reaches(?from, ?net) => ?from, ?net",
		Teaches: "recursion: transitive reach through R/L/ferrite/fuse",
	},
	{
		Label:   "Reachable within one series element",
		Query:   "net.reaches(?from, ?net, ?hops), ?hops <= 1 => ?from, ?net, ?hops",
		Teaches: "distance: ?hops binds the EXACT crossing count, so a radius is a comparison (writing 1 in that slot would mean exactly one hop, skipping the net itself)",
	},
	{
		Label:   "Reachable, with the route it took",
		Query:   "net.route(?from, ?net, ?path) => ?from, ?net, ?path",
		Teaches: "evidence: the same walk as reaches, binding what it crossed (`VBUS -> [R5] -> VBUS_F`), so an answer can be checked without re-asking it",
	},
	{
		Label:   "Power pins on a single-connection net",
		Query:   `pin.role(?ref, ?pin, "power"), pin.net(?ref, ?pin, ?net), net.pin_count(?net, ?c), ?c < 2 => ?ref, ?pin, ?net`,
		Teaches: "pin-level join: a pin, its role, and its net's fan-out",
	},
	{
		Label:   "Clock-source terminal nets (excluding ground)",
		Query:   `component.class(?y, "clock"), component.net(?y, ?net), not net.ground(?net) => ?y, ?net`,
		Teaches: "class + negation: pick a device family (clock covers oscillator/crystal/resonator), drop the grounded net",
	},
	{
		Label:   "Diode-family parts (diode, LED, or TVS)",
		Query:   `component.class(?ref, "diode") => ?ref`,
		Teaches: "class set membership: the diode family tag matches a plain diode, an LED, and a TVS",
	},
	{
		Label:   "Nets the design tool put in a class",
		Query:   "net.netclass(?net, ?class) => ?net, ?class",
		Teaches: "tool-assigned scope: the class the layout tool recorded (KiCad net_settings), distinct from a derived role like net.ground",
	},
	{
		Label:   "Signal nets clamped by a Zener at a connector",
		Query:   `component.class(?j, "connector"), component.net(?j, ?net), component.net(?z, ?net), component.class(?z, "zener") => ?net, ?z`,
		Teaches: "topology pattern: one net joining two device classes (the shape esd-clamp-not-tvs refines)",
	},
	{
		// Without this rung a reader finishes the catalog believing `not` over a single relation is
		// all the negation there is, and designs around a limitation that does not exist (agni issue
		// 522).
		Label:   "Nets with no test point on them",
		Query:   `has_test_point(?n) :- component.net(?tp, ?n), component.class(?tp, "test_point"); entity(?n, "net"), not has_test_point(?n) => ?n`,
		Teaches: "derived relation: `;` separates clauses and `:-` names one, which is how you negate a PAIR of relations — the shape of every \"X with no related Y\" question",
	},
	{
		Label:   "Rails above a part's recommended maximum",
		Query:   `component.mpn(?ref, ?mpn), param.range(?mpn, ?sym, "recommended_operating", ?min, ?max), component.net(?ref, ?net), net.nominal_voltage(?net, ?v), ?v > ?max => ?ref, ?net, ?v, ?max`,
		Teaches: "datasheet range: join a two-sided limit (by kind) against the design's rail voltage (needs --params)",
	},
}

// Examples returns the shared teaching-query catalog (WS14-002), in concept-ladder order. The web
// panel and the CLI both read this one source.
func Examples() []ExampleQuery {
	out := make([]ExampleQuery, len(examples))
	copy(out, examples)
	return out
}
