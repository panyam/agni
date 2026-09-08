package check

import (
	"fmt"
	"sort"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// DefaultTraceHops is the radius a trace searches to when the caller names none.
//
// Unlike ProtectionReachHops and its neighbours this number is a SEARCH BUDGET and not an
// electrical claim. Those radii encode a physical fact (a clamp six resistors away does not protect
// the pin), so widening one turns real findings into false passes. Nothing about a trace degrades
// with distance: a route eight crossings long is still the route. The bound is here so the walk
// terminates on a large board and so a "no route" answer is a statement someone can check, which is
// why the radius is printed with the answer and exposed as a flag rather than fixed here.
const DefaultTraceHops = 6

// traceStubLimit caps the parts reported as sitting on one net of the route. A signal net carries a
// handful; a route that lands on a rail carries hundreds, and printing them buries the route itself.
// What is dropped is COUNTED and reported (TraceNet.StubsElided), never silently truncated.
const traceStubLimit = 12

// Endpoint names one pin of one component: the ref-des and the pin designator on it.
type Endpoint struct {
	RefDes string `json:"ref_des"`
	Pin    string `json:"pin"`
}

// String renders the endpoint the way a person writes it, "U7.3".
func (e Endpoint) String() string { return e.RefDes + "." + e.Pin }

// TraceOutcome is what a trace answered. The three are kept apart because two of them are ordinary
// answers and one is a failure to ask the question at all.
type TraceOutcome string

const (
	// TraceRouted: a route was found within the radius.
	TraceRouted TraceOutcome = "routed"
	// TraceNoRoute: both endpoints resolved and no route joins them within the radius. A real
	// answer about the design.
	TraceNoRoute TraceOutcome = "no-route"
	// TraceUnresolved: an endpoint does not name anything the design has, so nothing was walked.
	// This must never render as "not connected": a pin named in a declaration that the design
	// spells differently is exactly what a typo produces, and reporting it as a disconnection
	// sends the reader to look at the board instead of at the declaration.
	TraceUnresolved TraceOutcome = "unresolved"
)

// TraceEnd is one resolved endpoint of a trace.
type TraceEnd struct {
	Endpoint
	// PinName is the functional name the part type declares for this pin ("SDA", "PTC11"), or ""
	// on a source carrying no part-type pin data. It is what a datasheet and a firmware header
	// call the pin, so it is what makes the endpoint recognisable to the person reading.
	PinName string `json:"pin_name,omitempty"`
	Net     string `json:"net"`
}

// TraceCross is one series element crossed on the route, with its own pins on each side.
type TraceCross struct {
	RefDes   string `json:"ref_des"`
	Class    string `json:"class,omitempty"`
	EnterPin string `json:"enter_pin,omitempty"`
	ExitPin  string `json:"exit_pin,omitempty"`
	FromNet  string `json:"from_net"`
	ToNet    string `json:"to_net"`
}

// TraceStub is a part sitting on a net of the route that the route does not pass through.
//
// These are reported rather than filtered, and that is deliberate. A test point on a path net is
// where a reviewer can put a probe, which is the most actionable thing on the line; a capacitor
// there is the filter someone is about to ask about. Reducing the output to the series elements
// removes exactly the parts a person reads a trace to find.
type TraceStub struct {
	RefDes string `json:"ref_des"`
	Pin    string `json:"pin"`
	Class  string `json:"class,omitempty"`
}

// TraceNet is one net on the route, with what else sits on it.
type TraceNet struct {
	Name  string      `json:"name"`
	Stubs []TraceStub `json:"stubs,omitempty"`
	// StubsElided is how many further parts sit on this net beyond the ones listed, so a capped
	// list reads as capped rather than as complete.
	StubsElided int `json:"stubs_elided,omitempty"`
	// BusLike marks a net the walk would refuse to continue through: a rail, a ground, or any
	// rail-scale fan-out. A route may END on one, so saying which net it was is what stops a
	// reader assuming the trace stopped early for some other reason.
	BusLike bool `json:"bus_like,omitempty"`
}

// Trace is one pin-to-pin question and its whole answer: the outcome, the route when there is one,
// and the radius the answer rests on.
type Trace struct {
	From    TraceEnd     `json:"from"`
	To      TraceEnd     `json:"to"`
	Outcome TraceOutcome `json:"outcome"`
	// Reason says why, for no-route and unresolved; empty for a route.
	Reason    string       `json:"reason,omitempty"`
	Radius    int          `json:"radius"`
	Crossings []TraceCross `json:"crossings,omitempty"`
	Nets      []TraceNet   `json:"nets,omitempty"`
}

// TracePins walks from one pin to another through series pass elements and returns the route with
// its outcome from a single call.
//
// Producing the answer and its evidence together is the discipline PullUpVerdict follows, for the
// same reason: a caller cannot reach a routed outcome without the route that justifies it, so there
// is no way to report a connection by forgetting the second step. It is also the artifact issue 518
// asks for, since every walk in the engine already held the route and discarded it on the way out.
//
// The walk admits a bus-like net as a destination (ReachToTerminus), because a device pin sitting on
// a rail is an ordinary endpoint, and still refuses to continue through one.
func TracePins(m Model, from, to Endpoint, hops int) Trace {
	if hops <= 0 {
		hops = DefaultTraceHops
	}
	t := Trace{Radius: hops}
	fromNet, fromEnd, reason := resolveEndpoint(m, from)
	t.From = fromEnd
	if reason != "" {
		t.Outcome, t.Reason = TraceUnresolved, reason
		return t
	}
	toNet, toEnd, reason := resolveEndpoint(m, to)
	t.To = toEnd
	if reason != "" {
		t.Outcome, t.Reason = TraceUnresolved, reason
		return t
	}

	// One net, no crossing. A routed answer with an empty crossing list, not a special case: the
	// two pins are the same electrical node, which is the strongest form of "connected" there is.
	if fromNet.Name == toNet.Name {
		t.Outcome = TraceRouted
		t.Nets = []TraceNet{traceNet(m, fromNet, nil, []Endpoint{from, to})}
		return t
	}

	r := m.ReachToTerminus(fromNet, hops)
	if _, ok := r.Parent[toNet.Name]; !ok {
		t.Outcome = TraceNoRoute
		t.Reason = fmt.Sprintf("no series path from %s to %s within %d crossings",
			fromNet.Name, toNet.Name, hops)
		return t
	}

	// The two views of one route, taken from the same walk: the nets in crossing order and the
	// steps between them. Step i joins nets[i] to nets[i+1], so nothing has to be re-derived by
	// searching for the net a step landed on.
	nets, steps := r.PathTo(toNet), r.StepsTo(toNet)
	onRoute := map[string][]string{}
	for i, s := range steps {
		t.Crossings = append(t.Crossings, TraceCross{
			RefDes: s.Through, Class: string(m.ComponentClass(s.Through)),
			EnterPin: s.FromPin, ExitPin: s.ToPin,
			FromNet: nets[i].Name, ToNet: nets[i+1].Name,
		})
		onRoute[nets[i].Name] = append(onRoute[nets[i].Name], s.Through)
		onRoute[nets[i+1].Name] = append(onRoute[nets[i+1].Name], s.Through)
	}
	ends := []Endpoint{from, to}
	for _, n := range nets {
		t.Nets = append(t.Nets, traceNet(m, n, onRoute[n.Name], ends))
	}
	t.Outcome = TraceRouted
	return t
}

// resolveEndpoint turns a named pin into the net it sits on, or returns the reason it could not.
//
// The reasons are separated because they send a reader to different places. A ref-des the design
// does not have is a wrong name; a declared pin on no net is a genuine disconnection at that pin; a
// pin the part type does not declare is a wrong pin; and a part whose pins were never read at all is
// a gap in the READ rather than anything about the design. Collapsing these into "not found" is what
// makes a typo in a declaration look like a disconnected board.
func resolveEndpoint(m Model, e Endpoint) (*ir.Net, TraceEnd, string) {
	end := TraceEnd{Endpoint: e}
	if e.RefDes == "" || e.Pin == "" {
		return nil, end, fmt.Sprintf("%q is not a pin: name it as <ref-des>.<pin>", e.String())
	}
	if !m.HasComponent(e.RefDes) {
		return nil, end, fmt.Sprintf("no component %s in this design", e.RefDes)
	}
	// "~" is KiCad's spelling of "this pin has no name", and it reaches the IR verbatim. Reporting
	// it would print `TP2.1 (~)`, so it is read as absent here, the same reading classifyPinRole
	// already takes of it.
	if pn := m.PinName(e.RefDes, e.Pin); pn != "~" {
		end.PinName = pn
	}
	n := netOfPin(m, e)
	if n == nil {
		switch {
		case m.PinDeclared(e.RefDes, e.Pin):
			return nil, end, fmt.Sprintf("%s is declared and sits on no net", e.String())
		case declaresAnyPin(m, e.RefDes):
			return nil, end, fmt.Sprintf("%s declares no pin %s", e.RefDes, e.Pin)
		default:
			return nil, end, fmt.Sprintf("%s is on no net, and the read supplied no pin list for %s",
				e.String(), e.RefDes)
		}
	}
	end.Net = n.Name
	return n, end, ""
}

// netOfPin finds the net carrying this exact (ref-des, pin) connection.
//
// By connection rather than through PinNetName, which answers with a NAME, and net names are not
// unique (Model.NetNameCount exists for that). A trace resolved by name could start its walk from a
// different net that happens to share the spelling, and every answer after that would be about the
// wrong node.
func netOfPin(m Model, e Endpoint) *ir.Net {
	for _, n := range m.Nets() {
		for _, c := range n.Connections {
			if c.ComponentRef == e.RefDes && c.PinRef == e.Pin {
				return n
			}
		}
	}
	return nil
}

// declaresAnyPin reports whether the read gave this component a part-type pin list at all, which is
// what separates "wrong pin name" from "this format carried no pins".
func declaresAnyPin(m Model, refDes string) bool {
	for _, p := range m.Pins() {
		if p.Component.GetRefDes() == refDes {
			return true
		}
	}
	return false
}

// traceNet describes one net of the route: everything on it that the route does not pass through,
// with the endpoints' own pins excluded because they are already named as the endpoints.
func traceNet(m Model, n *ir.Net, route []string, ends []Endpoint) TraceNet {
	tn := TraceNet{Name: n.Name, BusLike: IsBusLike(m, n)}
	skip := map[string]bool{}
	for _, ref := range route {
		skip[ref] = true
	}
	var stubs []TraceStub
	for _, c := range n.Connections {
		if skip[c.ComponentRef] || isEndpoint(ends, c.ComponentRef, c.PinRef) {
			continue
		}
		// A virtual power/flag symbol is connectivity evidence, not a part on the board, and a
		// stub list answers "what is sitting on this net" — the question IsVirtualRef exists to
		// keep those two apart. Listing them buries the real parts under a dozen #PWR entries on
		// every rail, and what they were evidence FOR is already reported as BusLike.
		if IsVirtualRef(c.ComponentRef) {
			continue
		}
		stubs = append(stubs, TraceStub{
			RefDes: c.ComponentRef, Pin: c.PinRef, Class: string(m.ComponentClass(c.ComponentRef)),
		})
	}
	// Test points first, then by ref-des, so the probe a reviewer is looking for is at the top of
	// the list rather than wherever the netlist happened to put it. Deterministic either way, which
	// a golden transcript needs.
	sort.SliceStable(stubs, func(i, j int) bool {
		ti, tj := stubs[i].Class == string(ClassTestPoint), stubs[j].Class == string(ClassTestPoint)
		if ti != tj {
			return ti
		}
		if stubs[i].RefDes != stubs[j].RefDes {
			return stubs[i].RefDes < stubs[j].RefDes
		}
		return stubs[i].Pin < stubs[j].Pin
	})
	if len(stubs) > traceStubLimit {
		tn.StubsElided = len(stubs) - traceStubLimit
		stubs = stubs[:traceStubLimit]
	}
	tn.Stubs = stubs
	return tn
}

func isEndpoint(ends []Endpoint, ref, pin string) bool {
	for _, e := range ends {
		if e.RefDes == ref && e.Pin == pin {
			return true
		}
	}
	return false
}
