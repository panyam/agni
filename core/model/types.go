package model

import (
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// ComponentClass is the device class of a placed component: the component.class fact
// (docs/19). Values are stable strings so rules and reports can match on them; a class the
// derivation cannot establish is ClassUnknown, never a guess, so class-quantified rules
// stay silent rather than misfire on unfamiliar designs.
type ComponentClass string

// The component.class vocabulary. ClassLED, ClassTVS, and ClassZener are deliberately distinct
// from ClassDiode (protection and indicator rules quantify over them separately), and
// ClassFerrite from ClassInductor, even though each is electrically a subtype. ClassZener is a
// clamp/reference, distinct from ClassTVS because a Zener is a slower clamp than a fast ESD TVS
// (esd-clamp-not-tvs, WS3-078, credits them differently).
// ClassTestConnector is distinct from ClassConnector: a debug / test / edge-card / programming
// connector is a bench interface, not a field-facing harness, so protection rules (esd,
// input-protection) that quantify over ClassConnector exclude it.
//
// ClassClock is the clock-source FAMILY (WS10-015); ClassOscillator, ClassCrystal, and
// ClassCeramicResonator are its subtypes. The family is deliberately NOT ClassCrystal: an active
// oscillator is-NOT-a crystal (it CONTAINS one), so a family-level clock rule must not answer
// HasClass(crystal) for it. A part's clock TYPE branches rules (an oscillator uses no external load
// caps; a ceramic resonator has them integrated; a bare crystal needs them). Crystal-vs-resonator is
// datasheet-driven — the vendor label is unreliable (swapped in the field) and the two are structurally
// alike — so the keyword/structural path only ever resolves the oscillator subtype or stays at the
// family; the crystal / ceramic_resonator subtype comes from a seeded device_class.
const (
	ClassResistor         ComponentClass = "resistor"
	ClassCapacitor        ComponentClass = "capacitor"
	ClassInductor         ComponentClass = "inductor"
	ClassFerrite          ComponentClass = "ferrite"
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
	ClassUnknown          ComponentClass = "unknown"
)

// PinRole is the semantic role of a pin, derived from its declared name within the
// component's device-class context (Model.PinRole).
type PinRole string

// The pin-role vocabulary. Polarity roles are assigned only within the diode family
// (diode, led, tvs) so a "K" pin on an IC never reads as a cathode; power/ground come
// from rail-name conventions on any class. RoleUnknown is the honest default — rules
// skip unknowns, never guess.
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
// netlist-only source with no part data yields none). It deliberately carries no
// direction — resolve it through Model.PinDir so every consumer sees the same
// last-section-wins value; membership is Model.PinConnected.
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

// BoardVia is one via. Annular returns the copper ring width around the drill,
// (Size - Drill) / 2 — the quantity the annular-width rule bounds.
type BoardVia struct {
	At    *geom.Point
	Size  int64
	Drill int64
}

// Annular is the copper ring width around the drill: (Size - Drill) / 2.
func (v BoardVia) Annular() int64 { return (v.Size - v.Drill) / 2 }

// Reach is a bounded series-walk neighborhood (WS3-011, the "reach" primitive): the nets
// reachable from a start net by crossing SERIES PASS ELEMENTS, in BFS order (start
// first), plus the ref-des set of the elements crossed. Protection and presence rules are
// reachability questions ("a fuse sits somewhere between the connector and the
// regulator"), and a series element by definition splits the schematic net, so a per-net
// rule cannot see across it. Parent records how each net was entered, for path extraction
// (PathTo / ThroughOnPath); the builder — the Model implementation's walk — populates it.
type Reach struct {
	Nets    []*ir.Net
	Crossed map[string]bool
	Parent  map[string]ReachStep // net name -> how it was entered
	// Depth is the number of series crossings from the start net, which the BFS knows as it
	// goes (the start net is 0, so the walk is reflexive at distance zero). Recorded rather
	// than re-derived by chasing Parent: that is O(path) per net, and where parallel passes
	// bridge the same two nets the chain is one of several equally-valid paths, so a derived
	// count can disagree with the count the walk actually used. Exposed because DISTANCE is
	// part of the question a reachability rule asks (WS3-112), not an implementation detail.
	Depth map[string]int // net name -> series crossings from the start
}

// ReachStep records how a net was reached during the series walk: the net crossed FROM and
// the pass element (ref-des) crossed THROUGH.
type ReachStep struct {
	From    string
	Through string
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
