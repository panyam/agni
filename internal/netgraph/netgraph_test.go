package netgraph

import (
	"reflect"
	"sort"
	"testing"
)

// Net.ID is a deterministic hash of the connection set: two electrically-distinct nets that share a
// NAME (the duplicate-net-name case) get distinct ids, a net with the same connectivity hashes the
// same across two solves, and a pinless net has an empty id (nothing to hash, falls back to name).
func TestNetID(t *testing.T) {
	// Three separate two-pin clusters, all labelled "DUP" — connect-by-name would merge same-label
	// clusters, so keep each on its own label ("DUP1".."DUP3") to stay electrically distinct, which
	// is the real duplicate-net-name shape (distinct nets a human named the same).
	build := func() []Net {
		wires := []Wire{
			{A: Point{0, 0}, B: Point{10, 0}, Label: "A"},
			{A: Point{0, 20}, B: Point{10, 20}, Label: "B"},
		}
		pins := []Pin{
			{At: Point{0, 0}, Comp: "R1", Pin: "1"}, {At: Point{10, 0}, Comp: "R2", Pin: "1"},
			{At: Point{0, 20}, Comp: "R3", Pin: "1"}, {At: Point{10, 20}, Comp: "R4", Pin: "1"},
		}
		nets, _, _ := Build(wires, nil, pins, nil)
		return nets
	}
	nets := build()
	byName := map[string]Net{}
	for _, n := range nets {
		byName[n.Name] = n
	}
	a, b := byName["A"], byName["B"]
	if a.ID == "" || b.ID == "" {
		t.Fatalf("nets with pins must have ids, got A=%q B=%q", a.ID, b.ID)
	}
	if a.ID == b.ID {
		t.Errorf("electrically-distinct nets must have distinct ids, both = %q", a.ID)
	}
	// Determinism: a second identical solve reproduces every id.
	for _, n := range build() {
		if n.ID != byName[n.Name].ID {
			t.Errorf("net %q id not deterministic: %q vs %q", n.Name, n.ID, byName[n.Name].ID)
		}
	}
	// A pinless (label-only) net has no connections to hash -> empty id.
	pinless, _, _ := Build([]Wire{{A: Point{0, 0}, B: Point{10, 0}, Label: "LONELY"}}, nil, nil, nil)
	for _, n := range pinless {
		if n.Name == "LONELY" && n.ID != "" {
			t.Errorf("pinless net should have empty id, got %q", n.ID)
		}
	}
}

// Two wire clusters carrying the same label must merge (connect-by-name), even when they never
// touch geometrically; ground taps at scattered points collapse to one net; an unlabelled
// cluster with pins gets a synthetic name; an unlabelled pinless node is dropped.
func TestBuildConnectByName(t *testing.T) {
	wires := []Wire{
		{A: Point{0, 0}, B: Point{0, 10}, Label: "VOUT"},
		{A: Point{100, 0}, B: Point{100, 10}, Label: "VOUT"}, // separate cluster, same name
		{A: Point{0, 50}, B: Point{0, 60}, Label: ""},        // unlabelled, will get a pin
		{A: Point{200, 0}, B: Point{200, 5}, Label: ""},      // unlabelled, pinless -> dropped
	}
	anchors := []Anchor{
		{At: Point{300, 0}, Label: "GND"},
		{At: Point{400, 0}, Label: "GND"}, // second ground tap, same net
	}
	pins := []Pin{
		{At: Point{0, 0}, Comp: "R1", Pin: "1"},    // on VOUT (first cluster)
		{At: Point{100, 10}, Comp: "R2", Pin: "2"}, // on VOUT (second cluster)
		{At: Point{0, 50}, Comp: "R3", Pin: "1"},   // on the unlabelled cluster
		{At: Point{300, 0}, Comp: "R4", Pin: "2"},  // on GND
		{At: Point{400, 0}, Comp: "R5", Pin: "1"},  // on GND
	}

	got := map[string][]Conn{}
	nets, _, _ := Build(wires, anchors, pins, nil)
	for _, n := range nets {
		got[n.Name] = n.Conns
	}

	want := map[string][]Conn{
		"VOUT": {{Comp: "R1", Pin: "1"}, {Comp: "R2", Pin: "2"}},
		"GND":  {{Comp: "R4", Pin: "2"}, {Comp: "R5", Pin: "1"}},
		"N$1":  {{Comp: "R3", Pin: "1"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Build() =\n  %v\nwant\n  %v", got, want)
	}
}

// BuildWithPoints reports which net every input point resolved to, so a caller that offset its
// points into meaningful bands (the KiCad hierarchy walk's sheet instances, WS9-028) can read
// membership back. Points a wire unions share a net; a pin's point maps to its net; an anchor's
// point maps to the net it named; a point on drawing noise (unnamed, pinless) is omitted, matching
// the wireNets rule.
func TestBuildWithPoints(t *testing.T) {
	wires := []Wire{
		{A: Point{0, 0}, B: Point{0, 10}, Label: "VOUT"},
		{A: Point{200, 0}, B: Point{200, 5}, Label: ""}, // unlabelled, pinless -> drawing noise
	}
	anchors := []Anchor{{At: Point{300, 0}, Label: "GND"}}
	pins := []Pin{
		{At: Point{0, 10}, Comp: "R1", Pin: "1"},  // wire-unioned onto VOUT
		{At: Point{300, 0}, Comp: "R4", Pin: "2"}, // on GND
	}

	_, _, _, pointNets := BuildWithPoints(wires, anchors, pins, nil)
	names := map[Point]string{}
	for pt, nr := range pointNets {
		names[pt] = nr.Name
	}
	want := map[Point]string{
		{0, 0}:   "VOUT",
		{0, 10}:  "VOUT", // the wire's far end and the pin there resolve to the same net
		{300, 0}: "GND",
	}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("pointNets names =\n  %v\nwant\n  %v", names, want)
	}
	// Points on the same net carry the same id; distinct nets carry distinct, non-empty ids.
	if id := pointNets[Point{0, 0}].ID; id == "" || id != pointNets[Point{0, 10}].ID {
		t.Errorf("VOUT points should share one non-empty id, got %q and %q", id, pointNets[Point{0, 10}].ID)
	}
	if pointNets[Point{300, 0}].ID == "" || pointNets[Point{300, 0}].ID == pointNets[Point{0, 0}].ID {
		t.Errorf("GND should have its own non-empty id, got %q (VOUT %q)", pointNets[Point{300, 0}].ID, pointNets[Point{0, 0}].ID)
	}
	if _, ok := pointNets[Point{200, 0}]; ok {
		t.Errorf("drawing-noise point {200,0} should be omitted, got %v", pointNets[Point{200, 0}])
	}
}

// A pin landing on the same grid point as another pin (direct pin-to-pin, no wire) shares a net.
func TestBuildDirectPinToPin(t *testing.T) {
	pins := []Pin{
		{At: Point{5, 5}, Comp: "J1", Pin: "1"},
		{At: Point{5, 5}, Comp: "J2", Pin: "3"},
	}
	nets, _, _ := Build(nil, nil, pins, nil)
	if len(nets) != 1 || len(nets[0].Conns) != 2 {
		t.Fatalf("want one net with two conns, got %+v", nets)
	}
}

// A driver anchor (PWR_FLAG) marks its net Driven; an external anchor (global/hierarchical label,
// power rail) marks it External; a plain net carries neither.
func TestBuildDrivenExternal(t *testing.T) {
	wires := []Wire{
		{A: Point{0, 0}, B: Point{10, 0}},   // net with the flag/rail anchor
		{A: Point{0, 50}, B: Point{10, 50}}, // plain net
	}
	pins := []Pin{
		{At: Point{0, 0}, Comp: "U1", Pin: "1"},
		{At: Point{0, 50}, Comp: "U2", Pin: "1"},
		{At: Point{10, 50}, Comp: "U3", Pin: "1"},
	}
	anchors := []Anchor{{At: Point{10, 0}, Label: "+5V", Driver: true, External: true}}

	nets, _, _ := Build(wires, anchors, pins, nil)
	by := map[string]Net{}
	for _, n := range nets {
		by[n.Name] = n
	}
	if rail := by["+5V"]; !rail.Driven || !rail.External {
		t.Errorf("+5V driven=%v external=%v, want both true", rail.Driven, rail.External)
	}
	// the plain net (unnamed, two pins) is neither.
	for _, n := range nets {
		if n.Name != "+5V" && (n.Driven || n.External) {
			t.Errorf("net %q driven=%v external=%v, want both false", n.Name, n.Driven, n.External)
		}
	}
}

// A wire endpoint on nothing is dangling; ends met by a pin, an anchor, a junction, or a second
// wire endpoint are not. Carries every non-dangling exclusion so the degree/occupancy logic is
// pinned in one place.
func TestBuildDangling(t *testing.T) {
	wires := []Wire{
		{A: Point{0, 0}, B: Point{10, 0}, Id: "w1"},   // (0,0) floats -> dangle
		{A: Point{10, 0}, B: Point{10, 10}, Id: "w2"}, // (10,0) met by w1 (deg 2); (10,10) -> pin
		{A: Point{20, 0}, B: Point{20, 10}, Id: "w3"}, // (20,0) floats -> dangle; (20,10) -> junction
		{A: Point{30, 0}, B: Point{40, 0}, Id: "w4"},  // (30,0) floats -> dangle; (40,0) -> anchor
	}
	pins := []Pin{{At: Point{10, 10}, Comp: "R1", Pin: "1"}}
	anchors := []Anchor{{At: Point{40, 0}, Label: "VCC"}}
	junctions := []Point{{20, 10}}

	_, dangles, _ := Build(wires, anchors, pins, junctions)

	var got []Point
	for _, d := range dangles {
		got = append(got, d.At)
	}
	want := []Point{{0, 0}, {20, 0}, {30, 0}} // sorted by X then Y
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dangling = %v, want %v", got, want)
	}
	if dangles[0].WireId != "w1" {
		t.Errorf("dangle at (0,0) WireId = %q, want w1", dangles[0].WireId)
	}
}

// A node carrying two labels aliases them: every cluster under EITHER name folds into one
// net. Before label-union, merging used only each node's single chosen name, so the
// two-label node joined one group and abandoned the other (the hierarchy walk relies on a
// local-labelled node still folding into the rail its power tap names).
func TestBuildLabelAliasUnion(t *testing.T) {
	wires := []Wire{
		{A: Point{0, 0}, B: Point{10, 0}},    // node with BOTH labels
		{A: Point{100, 0}, B: Point{110, 0}}, // cluster named only A
		{A: Point{200, 0}, B: Point{210, 0}}, // cluster named only B
	}
	anchors := []Anchor{
		{At: Point{0, 0}, Label: "A"},
		{At: Point{10, 0}, Label: "B"},
		{At: Point{100, 0}, Label: "A"},
		{At: Point{200, 0}, Label: "B"},
	}
	pins := []Pin{
		{At: Point{110, 0}, Comp: "R1", Pin: "1"},
		{At: Point{210, 0}, Comp: "R2", Pin: "1"},
	}
	nets, _, _ := Build(wires, anchors, pins, nil)
	if len(nets) != 1 {
		t.Fatalf("aliased labels must yield one net, got %+v", nets)
	}
	if len(nets[0].Conns) != 2 {
		t.Errorf("merged net conns = %+v, want R1.1 and R2.1", nets[0].Conns)
	}
}

// Anchor.Rank picks the net's name: the lowest rank wins regardless of input order, and
// equal ranks keep first-appearance order (the existing behavior, since every legacy
// caller leaves Rank at zero). The hierarchy walk names a hier-joined net by its rank-1
// path name, never by the rank-2 synthetic join label.
func TestBuildAnchorRank(t *testing.T) {
	wires := []Wire{{A: Point{0, 0}, B: Point{10, 0}}}
	anchors := []Anchor{
		{At: Point{0, 0}, Label: "\x00join", Rank: 2},
		{At: Point{10, 0}, Label: "/child/SIG", Rank: 1},
	}
	pins := []Pin{{At: Point{0, 0}, Comp: "R1", Pin: "1"}}
	nets, _, _ := Build(wires, anchors, pins, nil)
	if len(nets) != 1 || nets[0].Name != "/child/SIG" {
		t.Fatalf("nets = %+v, want one net named /child/SIG", nets)
	}
}

// TestBuildAliases: a net reports EVERY distinct label that landed on it, with rank, so
// naming-conflict rules can see what the naming pass collapsed. Single-labeled and
// unnamed nets carry no alias list.
func TestBuildAliases(t *testing.T) {
	wires := []Wire{
		{A: Point{0, 0}, B: Point{10, 0}},
		{A: Point{100, 0}, B: Point{110, 0}},
	}
	anchors := []Anchor{
		{At: Point{0, 0}, Label: "+3V3", Rank: 0},
		{At: Point{10, 0}, Label: "+3.3V", Rank: 0}, // second rail tap, same net
		{At: Point{10, 0}, Label: "PWR_A", Rank: 1}, // local alias on the rail
		{At: Point{100, 0}, Label: "SOLO", Rank: 1},
	}
	pins := []Pin{
		{At: Point{0, 0}, Comp: "U1", Pin: "1"},
		{At: Point{100, 0}, Comp: "U2", Pin: "1"},
	}
	nets, _, _ := Build(wires, anchors, pins, nil)
	byName := map[string]Net{}
	for _, n := range nets {
		byName[n.Name] = n
	}
	rail := byName["+3V3"]
	var got []string
	for _, a := range rail.Aliases {
		got = append(got, a.Name)
	}
	sort.Strings(got)
	if want := []string{"+3.3V", "+3V3", "PWR_A"}; !reflect.DeepEqual(got, want) {
		t.Errorf("rail aliases = %v, want %v", got, want)
	}
	ranks := map[string]int{}
	for _, a := range rail.Aliases {
		ranks[a.Name] = a.Rank
	}
	if ranks["+3.3V"] != 0 || ranks["PWR_A"] != 1 {
		t.Errorf("alias ranks = %v, want +3.3V:0 PWR_A:1", ranks)
	}
	if n := byName["SOLO"]; len(n.Aliases) != 1 || n.Aliases[0].Name != "SOLO" {
		t.Errorf("single-labeled net aliases = %v, want just itself", n.Aliases)
	}
}

// TestBuildWireNets: Build's third return maps each identified wire (by Id) to its solved
// net name — a labeled wire to the label, an unlabeled pinned wire to its N$ stub — and
// omits wires with no id or on drawing-noise nodes.
func TestBuildWireNets(t *testing.T) {
	wires := []Wire{
		{A: Point{0, 0}, B: Point{10, 0}, Label: "VOUT", Id: "w1"},
		{A: Point{0, 50}, B: Point{10, 50}, Id: "w2"}, // no label; a pin makes it a stub net
		{A: Point{0, 99}, B: Point{5, 99}, Id: "w3"},  // unlabeled, pinless -> dropped, omitted
	}
	pins := []Pin{{At: Point{0, 0}, Comp: "R1", Pin: "1"}, {At: Point{0, 50}, Comp: "R2", Pin: "1"}}
	_, _, wn := Build(wires, nil, pins, nil)
	if wn["w1"].Name != "VOUT" {
		t.Errorf("w1 -> %q, want VOUT", wn["w1"].Name)
	}
	if wn["w2"].Name != "N$1" {
		t.Errorf("w2 -> %q, want its N$ stub name", wn["w2"].Name)
	}
	if _, ok := wn["w3"]; ok {
		t.Errorf("w3 (drawing noise) must be omitted, got %q", wn["w3"].Name)
	}
}
