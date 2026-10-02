package model

import (
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// ComponentClass is the device class of a placed component, the component.class fact
// (docsite/content/architecture/rules-and-checks.md#the-fact-base-and-querying-it). Values are
// stable strings so rules and reports can match on them. A class the derivation cannot establish is
// ClassUnknown, never a guess, so class-quantified rules stay silent on unfamiliar designs.
type ComponentClass string

// The component.class vocabulary. ClassLED, ClassTVS, and ClassZener are distinct from ClassDiode
// (protection and indicator rules quantify over them separately), and ClassFerrite from
// ClassInductor, even though each is electrically a subtype. ClassZener is distinct from ClassTVS
// because a Zener is a slower clamp than a fast ESD TVS (esd-clamp-not-tvs, WS3-078, credits them
// differently). ClassTestConnector is distinct from ClassConnector because a debug, test, edge-card
// or programming connector is a bench interface, not a field-facing harness, so protection rules
// (esd, input-protection) that quantify over ClassConnector exclude it. A board-to-board connector
// is NOT a class, because whether it faces the field depends on the product rather than the part; a
// design declares it in its intent instead (Model.ExposedConnector, agni issue 831).
//
// ClassClock is the clock-source FAMILY (WS10-015), with ClassOscillator, ClassCrystal, and
// ClassCeramicResonator as its subtypes. The family is NOT ClassCrystal, since an active oscillator
// CONTAINS a crystal rather than being one, so a family-level clock rule must not answer
// HasClass(crystal) for it. A part's clock TYPE branches rules (an oscillator uses no external load
// caps, a ceramic resonator has them integrated, a bare crystal needs them). Crystal-vs-resonator is
// datasheet-driven, because the vendor label is unreliable and the two are structurally alike. So
// the keyword/structural path only resolves the oscillator subtype or stays at the family, and the
// crystal / ceramic_resonator subtype comes from a seeded device_class.
const (
	ClassResistor  ComponentClass = "resistor"
	ClassCapacitor ComponentClass = "capacitor"
	ClassInductor  ComponentClass = "inductor"
	ClassFerrite   ComponentClass = "ferrite"
	// ClassThermistor is a temperature-dependent resistor (NTC inrush limiters, PTC resettable fuses,
	// temperature sense elements). Its family is ClassResistor, because it is a two-terminal resistor
	// for every topological question and not for anything temperature-related, the same split
	// ClassFerrite makes against ClassInductor. Without it a thermistor emits no component.class row
	// and drops out of every class-scoped rule; on one real board that was 15 parts (agni issue 627).
	ClassThermistor       ComponentClass = "thermistor"
	ClassDiode            ComponentClass = "diode"
	ClassLED              ComponentClass = "led"
	ClassTVS              ComponentClass = "tvs"
	ClassZener            ComponentClass = "zener"
	ClassFuse             ComponentClass = "fuse"
	ClassConnector        ComponentClass = "connector"
	ClassTestConnector    ComponentClass = "test_connector"
	ClassTestPoint        ComponentClass = "test_point"
	ClassClock            ComponentClass = "clock"
	ClassOscillator       ComponentClass = "oscillator"
	ClassCrystal          ComponentClass = "crystal"
	ClassCeramicResonator ComponentClass = "ceramic_resonator"
	ClassIC               ComponentClass = "ic"
	ClassTransistor       ComponentClass = "transistor"
	// ClassIdealDiodeController is a controller that drives an external FET to behave as a diode
	// (ORing controllers, ideal-diode controllers, power muxes). No netlist labels a FET plus bias
	// network as an ideal diode, so reverse-blocking analysis takes the identity from a seeded
	// datasheet or reports that it does not know (agni issue 74). It is DATASHEET-DRIVEN like the
	// crystal / ceramic_resonator split, reached through deviceClassAliases from a seeded
	// device_class rather than from a refdes prefix or a name keyword.
	ClassIdealDiodeController ComponentClass = "ideal_diode_controller"
	ClassUnknown              ComponentClass = "unknown"
)

// ComponentClasses is every class in the vocabulary above except ClassUnknown, in declaration order.
// Anything that enumerates the vocabulary reads this rather than keeping its own list (agni issue
// 677). Go cannot enumerate constants, so TestComponentClassesListsEveryConstant reads the const
// block and holds this list to it.
func ComponentClasses() []ComponentClass {
	return []ComponentClass{
		ClassResistor, ClassCapacitor, ClassInductor, ClassFerrite, ClassThermistor,
		ClassDiode, ClassLED, ClassTVS, ClassZener, ClassFuse,
		ClassConnector, ClassTestConnector, ClassTestPoint,
		ClassClock, ClassOscillator, ClassCrystal, ClassCeramicResonator,
		ClassIC, ClassTransistor, ClassIdealDiodeController,
	}
}

// PinRole is the semantic role of a pin, derived from its declared name within the
// component's device-class context (Model.PinRole).
type PinRole string

// The pin-role vocabulary. Polarity roles are assigned only within the diode family
// (diode, led, tvs) so a "K" pin on an IC never reads as a cathode; power/ground come
// from rail-name conventions on any class. RoleUnknown is the default, and rules skip
// unknowns rather than guess.
const (
	RoleAnode   PinRole = "anode"
	RoleCathode PinRole = "cathode"
	RolePower   PinRole = "power"
	RoleGround  PinRole = "ground"
	// Transistor terminals (WS3-117), assigned only within the transistor class for the same reason
	// the polarity roles are diode-only: a bare "G", "S" or "D" pin name means something on almost
	// every part, so an ungated match would mis-role most of a design.
	RoleGate    PinRole = "gate"
	RoleSource  PinRole = "source"
	RoleDrain   PinRole = "drain"
	RoleUnknown PinRole = "unknown"
)

// PinInst is one part-type pin of one placed component: the entity pin-level rules
// quantify over. It exists only for components whose part type declares pins (a
// netlist-only source with no part data yields none). It carries no direction, so
// every consumer resolves it through Model.PinDir and sees the same last-section-wins
// value; membership is Model.PinConnected.
type PinInst struct {
	Component  *ir.Component
	Designator string
}

// PinNetConflict is one malformed-input pin: it appears in more than one net's
// connections, which the pins-to-net many-to-one invariant forbids. Nets lists every
// claiming net in design order; Prov locates the second claim (the first place the
// input is provably wrong).
type PinNetConflict struct {
	RefDes string
	Pin    string
	Nets   []string
	Prov   *ir.Provenance
}

// BoardNet is one net's routed copper: the entity the geometric rules quantify over,
// with findings aggregating per net (copper primitives have no stable identity of their
// own). Net is the join key to ir.Net.name.
type BoardNet struct {
	Net      string
	Segments []BoardSeg
	Vias     []BoardVia
}

// BoardSeg is one routed track segment. Coordinates and width are in the sidecar's
// units (nanometers for the KiCad producer).
type BoardSeg struct {
	Layer string
	A, B  *geom.Point
	Width int64
}

// BoardVia is one via. Annular returns the copper ring width around the drill, which is
// the quantity the annular-width rule bounds.
type BoardVia struct {
	At    *geom.Point
	Size  int64
	Drill int64
}

// Annular is the copper ring width around the drill: (Size - Drill) / 2.
func (v BoardVia) Annular() int64 { return (v.Size - v.Drill) / 2 }

// Reach is a bounded series-walk neighborhood (WS3-011), the nets reachable from a start net
// by crossing SERIES PASS ELEMENTS, in BFS order (start first), plus the ref-des set of the
// elements crossed. See Model.Reach for why protection rules need it. Parent records how each
// net was entered, for PathTo / ThroughOnPath, and the Model implementation's walk populates it.
type Reach struct {
	Nets    []*ir.Net
	Crossed map[string]bool
	Parent  map[string]ReachStep // net name -> how it was entered
	// Depth is the number of series crossings from the start net as the BFS recorded it (the
	// start net is 0, so the walk is reflexive at distance zero). Do not re-derive it by chasing
	// Parent, because where parallel passes bridge the same two nets the chain is one of several
	// paths and its length can disagree with the walk's. Reachability rules read it (WS3-112).
	Depth map[string]int // net name -> series crossings from the start
}

// ReachStep records how a net was reached during the series walk: the net crossed FROM, the pass
// element (ref-des) crossed THROUGH, and that element's own pin on each side of the crossing.
//
// The pins let a step render as "R5.1 to R5.2" rather than a bare component name. They come off
// ir.Connection.pin_ref on each net. Either may be empty on a source whose connections carry no pin
// reference, and a renderer must handle that.
type ReachStep struct {
	From    string
	Through string
	FromPin string // Through's pin on the From net
	ToPin   string // Through's pin on the net this step reached
}

// PathTo returns the series path from the walk's start net to target, in crossing order
// (start first, target last); nil when target was not reached. Where parallel passes exist
// (two resistors bridging the same two nets) any of them is equally on the path for a
// protection question.
func (r Reach) PathTo(target *ir.Net) []*ir.Net {
	if target == nil || len(r.Nets) == 0 {
		return nil
	}
	byName := make(map[string]*ir.Net, len(r.Nets))
	for _, n := range r.Nets {
		byName[n.Name] = n
	}
	if byName[target.Name] == nil {
		return nil
	}
	var rev []*ir.Net
	at := target.Name
	for {
		rev = append(rev, byName[at])
		s, ok := r.Parent[at]
		if !ok {
			break // reached the start
		}
		at = s.From
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

// ThroughOnPath returns the pass elements crossed on the path from the start to target,
// in crossing order; nil when the target was not reached (or is the start).
func (r Reach) ThroughOnPath(target *ir.Net) []string {
	if target == nil {
		return nil
	}
	var rev []string
	at := target.Name
	for {
		s, ok := r.Parent[at]
		if !ok {
			break
		}
		rev = append(rev, s.Through)
		at = s.From
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	if len(rev) == 0 && at != target.Name {
		return nil
	}
	return rev
}

// StepsTo returns the crossings from the walk's start to target, in crossing order; nil when the
// target was not reached or IS the start. It is ThroughOnPath with the whole step kept, pins
// included, for a caller reporting the route (agni issue 518). ThroughOnPath stays for the callers
// asking a membership question over the path ("is a fuse on it").
func (r Reach) StepsTo(target *ir.Net) []ReachStep {
	if target == nil {
		return nil
	}
	var rev []ReachStep
	at := target.Name
	for {
		s, ok := r.Parent[at]
		if !ok {
			break
		}
		rev = append(rev, s)
		at = s.From
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

// routeArrow separates the hops of a rendered route. RenderRoute brackets each part because nets and
// ref-des are not distinguishable by shape on a real board (one sample names its nets N$1 and N$6 and
// its parts R1 and L1), so `N$6 -> [L1] -> N$1` reads without counting positions.
const routeArrow = " -> "

// RouteLine renders the route from the walk's start to target as one line, naming the nets it passed
// through and the part crossed between each pair:
//
//	VBUS -> [R5] -> VBUS_F -> [L1] -> VDD_3V3
//
// It is the fourth reading of one walk, beside PathTo (the nets), ThroughOnPath (the parts) and
// StepsTo (both, with pins), for when the rendering IS the answer, so a query can bind a route as a
// value and a table can carry it in a cell (agni issue 518).
//
// It returns one of three things:
//
//   - a route, when target was reached across one or more crossings
//   - target's own name, when target IS the start, since the two points are one electrical node
//   - "", when target was not reached
//
// Pins are left out to keep a table column narrow; StepsTo has them.
func (r Reach) RouteLine(target *ir.Net) string {
	if target == nil {
		return ""
	}
	if _, reached := r.Depth[target.Name]; !reached {
		return ""
	}
	// StepsTo records the net a step came FROM and the rendering names the net it arrives AT, so
	// step i arrives at step i+1's From, and the last one arrives at the target.
	steps := r.StepsTo(target)
	if len(steps) == 0 {
		return target.Name // the target IS the walk's start: one node, no crossings
	}
	hops := make([]RouteHop, len(steps))
	for i, s := range steps {
		to := target.Name
		if i+1 < len(steps) {
			to = steps[i+1].From
		}
		hops[i] = RouteHop{Through: s.Through, To: to}
	}
	return RenderRoute(steps[0].From, hops)
}

// RouteHop is one crossing of a rendered route: the part gone THROUGH and the net it arrives at.
type RouteHop struct {
	Through string
	To      string
}

// RenderRoute is the ONE renderer for a series route, and every surface that prints one calls it.
// The rendered string is a format callers depend on (DECISIONS.md, "A path is not a query column"),
// so a second implementation, such as the IO-map rules rendering off a check.Trace, must call this
// rather than copy it (#664). It takes plain hops so neither caller's walk type leaks in.
func RenderRoute(first string, hops []RouteHop) string {
	parts := make([]string, 0, 2*len(hops)+1)
	parts = append(parts, first)
	for _, h := range hops {
		parts = append(parts, "["+h.Through+"]", h.To)
	}
	return strings.Join(parts, routeArrow)
}
