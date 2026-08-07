package query

// ExampleQuery is one runnable teaching query (WS14-002): a plain-language intent, the datalog text
// that answers it, and the one concept it introduces. It is the shared catalog behind BOTH the web
// panel's click-to-run examples and the CLI's `agni query --examples`, so the two surfaces never
// drift — a user learns the same set from either.
type ExampleQuery struct {
	Label   string // plain-language intent, shown on the chip ("Parts on a rail above 3V")
	Query   string // the datalog text the chip fills and runs
	Teaches string // the one concept this rung introduces ("join", "filter", ...)
}

// examples is the concept ladder, in teaching order: each rung adds exactly one idea over the last
// (projection → filter → join → predicate → recursion). The set is design-independent, but ordered
// so the first (component-on-net) returns rows on any netlist while later rungs may return none on a
// design that lacks the data — an honest "no results" is itself a lesson (WS14-001).
var examples = []ExampleQuery{
	{
		Label:   "Every part on every net",
		Query:   "component-on-net(?ref, ?net) => ?ref, ?net",
		Teaches: "projection: => picks the answer columns",
	},
	{
		Label:   "Rails above 3V",
		Query:   "net.max_voltage(?net, ?v), ?v > 3 => ?net, ?v",
		Teaches: "filter: a bare comparison prunes rows",
	},
	{
		Label:   "Parts sitting on a rail above 3V",
		Query:   "component-on-net(?ref, ?net), net.max_voltage(?net, ?v), ?v > 3 => ?ref, ?net, ?v",
		Teaches: "join: a shared ?variable connects two relations",
	},
	{
		Label:   "Parts on USB nets",
		Query:   `component-on-net(?ref, ?net), contains(?net, "USB") => ?ref, ?net`,
		Teaches: "predicate: a string test over a bound value",
	},
	{
		Label:   "Reachable through series pass elements",
		Query:   "reaches(?from, ?net) => ?from, ?net",
		Teaches: "recursion: transitive reach through R/L/ferrite/fuse",
	},
	{
		Label:   "Reachable within one series element",
		Query:   "reaches(?from, ?net, ?hops), ?hops <= 1 => ?from, ?net, ?hops",
		Teaches: "distance: ?hops binds the EXACT crossing count, so a radius is a comparison (writing 1 in that slot would mean exactly one hop, skipping the net itself)",
	},
	{
		Label:   "Power pins on a single-connection net",
		Query:   `pin.role(?ref, ?pin, "power"), pin.net(?ref, ?pin, ?net), net.pin_count(?net, ?c), ?c < 2 => ?ref, ?pin, ?net`,
		Teaches: "pin-level join: a pin, its role, and its net's fan-out",
	},
	{
		Label:   "Clock-source terminal nets (excluding ground)",
		Query:   `component.class(?y, "clock"), component-on-net(?y, ?net), not net.ground(?net) => ?y, ?net`,
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
		Query:   `component.class(?j, "connector"), component-on-net(?j, ?net), component-on-net(?z, ?net), component.class(?z, "zener") => ?net, ?z`,
		Teaches: "topology pattern: one net joining two device classes (the shape esd-clamp-not-tvs refines)",
	},
	{
		Label:   "Rails above a part's recommended maximum",
		Query:   `component.mpn(?ref, ?mpn), param.range(?mpn, ?sym, "recommended_operating", ?min, ?max), component-on-net(?ref, ?net), net.nominal_voltage(?net, ?v), ?v > ?max => ?ref, ?net, ?v, ?max`,
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
