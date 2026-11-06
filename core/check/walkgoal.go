package check

import (
	"errors"
	"fmt"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// The goal questions a walk takes from a net it reached (agni issue 1340).
const (
	// WalkGoalPin asks for the series route to one pin, as agni trace walks it.
	WalkGoalPin = "pin"
	// WalkGoalPullUp asks for every pull-up resistor that takes the net to a rail, as the I2C pull-up
	// rule finds them.
	WalkGoalPullUp = "pull-up"
	// WalkGoalRail asks for the series route to the nearest rail-named net.
	WalkGoalRail = "rail"
)

// ErrUnknownWalkGoal is the error for a goal kind other than the three above.
var ErrUnknownWalkGoal = errors.New("unknown walk goal")

// WalkGoal is a goal question asked from a walk. At names the net it starts from, which must be a
// net the walk reached; empty takes the start's own net. To is the pin a pin goal routes to. Hops
// bounds a pin or rail goal's route from At, and 0 takes DefaultTraceHops. A pull-up goal searches
// as far as the net's pull-up reach, as the pull-up rule does.
type WalkGoal struct {
	Kind string
	At   string
	To   Endpoint
	Hops int
}

// WalkRoute is one route a goal found. It runs from the walk's start, along the walk's own path to
// the net the question was asked from, and on to the goal, so it extends the walk. EndsAt is the net
// the route reaches. Note says how a route with no crossing reached its goal, such as a pull-up
// that a part's datasheet puts inside the part.
type WalkRoute struct {
	Crossings []TraceCross `json:"crossings,omitempty"`
	Nets      []TraceNet   `json:"nets,omitempty"`
	EndsAt    string       `json:"ends_at"`
	Note      string       `json:"note,omitempty"`
}

// WalkGoalAnswer is a goal question's answer. It keeps trace's three outcomes apart. A route is
// TraceRouted with Routes set. No route within Radius is TraceNoRoute. A goal naming nothing (a
// net the walk never reached, or a pin the design lacks) is TraceUnresolved. Reason says why for
// the last two.
type WalkGoalAnswer struct {
	Kind    string       `json:"kind"`
	At      string       `json:"at"`
	To      TraceEnd     `json:"to,omitempty"`
	Outcome TraceOutcome `json:"outcome"`
	Reason  string       `json:"reason,omitempty"`
	Radius  int          `json:"radius"`
	Routes  []WalkRoute  `json:"routes,omitempty"`
}

// goalLeg is the part of a route past the net the question was asked from.
type goalLeg struct {
	nets      []*ir.Net
	crossings []TraceCross
	note      string
}

// WalkGoalFrom answers a goal question from a net the walk reached. An unknown kind is
// ErrUnknownWalkGoal. Every other failure is an answer with its outcome set.
//
// A pin goal asked from a pin start's own net answers what TracePins answers for the same two pins.
func WalkGoalFrom(m Model, w Walk, g WalkGoal) (*WalkGoalAnswer, error) {
	switch g.Kind {
	case WalkGoalPin, WalkGoalPullUp, WalkGoalRail:
	default:
		return nil, fmt.Errorf("%w %q: want %s, %s or %s", ErrUnknownWalkGoal, g.Kind, WalkGoalPin, WalkGoalPullUp, WalkGoalRail)
	}
	a := &WalkGoalAnswer{Kind: g.Kind, At: g.At, To: TraceEnd{Endpoint: g.To}, Radius: g.Hops}
	if a.Radius <= 0 {
		a.Radius = DefaultTraceHops
	}
	unresolved := func(why string) (*WalkGoalAnswer, error) {
		a.Outcome, a.Reason = TraceUnresolved, why
		return a, nil
	}
	if w.Reason != "" {
		return unresolved("the walk did not start: " + w.Reason)
	}
	byName := map[string]*ir.Net{}
	for _, n := range m.Nets() {
		if _, ok := byName[n.Name]; !ok {
			byName[n.Name] = n
		}
	}
	if a.At == "" {
		a.At = defaultGoalNet(m, w, byName)
	}
	walked := map[string]WalkNet{}
	for _, n := range w.Nets {
		walked[n.Net.Name] = n
	}
	if _, ok := walked[a.At]; !ok || byName[a.At] == nil {
		return unresolved(fmt.Sprintf("the walk did not reach a net %s; ask from a net it reached", a.At))
	}
	at := byName[a.At]
	ends := []Endpoint{w.From.Endpoint}

	var legs []goalLeg
	switch g.Kind {
	case WalkGoalPin:
		toNet, toEnd, why := resolveEndpoint(m, g.To)
		a.To = toEnd
		if why != "" {
			return unresolved(why)
		}
		ends = append(ends, toEnd.Endpoint)
		if toNet == at {
			legs = []goalLeg{{nets: []*ir.Net{at}}}
		} else if nets, crossings, ok := seriesRoute(m, at, toNet, a.Radius); ok {
			legs = []goalLeg{{nets: nets, crossings: crossings}}
		} else {
			a.Outcome = TraceNoRoute
			a.Reason = fmt.Sprintf("no series path from %s to %s within %d crossings", at.Name, toNet.Name, a.Radius)
			return a, nil
		}
	case WalkGoalRail:
		legs = railLeg(m, at, a.Radius)
		if legs == nil {
			a.Outcome = TraceNoRoute
			a.Reason = fmt.Sprintf("no rail is reachable from %s through series parts within %d crossings", at.Name, a.Radius)
			return a, nil
		}
	case WalkGoalPullUp:
		a.Radius = m.PullUpReachLimit(at.Name)
		legs = pullUpLegs(m, at, byName)
		if legs == nil {
			a.Outcome = TraceNoRoute
			a.Reason = fmt.Sprintf("no rail is reachable from %s through a fitted resistor within %d hops", at.Name, a.Radius)
			return a, nil
		}
	}

	prefixNets, prefix := walkPathTo(w, walked, a.At)
	for _, leg := range legs {
		crossings := append(append([]TraceCross{}, prefix...), leg.crossings...)
		var nets []*ir.Net
		for _, name := range prefixNets {
			nets = append(nets, byName[name])
		}
		nets = append(nets, leg.nets[1:]...)
		a.Routes = append(a.Routes, WalkRoute{
			Crossings: crossings,
			Nets:      routeNets(m, nets, crossings, ends),
			EndsAt:    nets[len(nets)-1].Name,
			Note:      leg.note,
		})
	}
	a.Outcome = TraceRouted
	return a, nil
}

// defaultGoalNet is the net a goal asks from when the request names none. A pin start's net, or for
// a part start, its first own net that is not a rail, a ground or a global net, since a goal asked
// from a rail answers nothing a reader wanted.
func defaultGoalNet(m Model, w Walk, byName map[string]*ir.Net) string {
	if w.From.Net != "" {
		return w.From.Net
	}
	first := ""
	for _, n := range w.Nets {
		if n.Hop != 1 {
			continue
		}
		if first == "" {
			first = n.Net.Name
		}
		if net := byName[n.Net.Name]; net != nil && !m.IsRailNet(net) && !IsBusLike(m, net) {
			return n.Net.Name
		}
	}
	return first
}

// walkPathTo follows the walk's entries back from net to its start. It returns the nets from a hop-1
// net to net, and the crossings between them, both in walk order.
func walkPathTo(w Walk, walked map[string]WalkNet, net string) ([]string, []TraceCross) {
	var nets []string
	var crossings []TraceCross
	for at := net; ; {
		nets = append([]string{at}, nets...)
		n := walked[at]
		if n.Hop <= 1 || n.Entered.FromNet == "" {
			break
		}
		crossings = append([]TraceCross{n.Entered}, crossings...)
		at = n.Entered.FromNet
	}
	return nets, crossings
}

// railLeg routes from at to the nearest rail-named net through series parts, as trace walks. A net
// that is a rail itself is its own route. Nil when no rail lies within hops.
func railLeg(m Model, at *ir.Net, hops int) []goalLeg {
	if m.IsRailNet(at) {
		return []goalLeg{{nets: []*ir.Net{at}}}
	}
	r := m.ReachToTerminus(at, hops)
	// Reach lists the nets in the order the walk reached them, so the first rail is the nearest.
	for _, n := range r.Nets[1:] {
		if m.IsRailNet(n) {
			nets, crossings, _ := seriesRoute(m, at, n, hops)
			return []goalLeg{{nets: nets, crossings: crossings}}
		}
	}
	return nil
}

// pullUpLegs routes from at through each pull-up the pull-up rule finds, in the rule's order. A part
// whose datasheet puts the pull-up inside it is a route of one net, with the witness as its note.
// Nil when there is neither.
func pullUpLegs(m Model, at *ir.Net, byName map[string]*ir.Net) []goalLeg {
	paths := AllPullUpPathsToRail(m, at)
	if len(paths) == 0 {
		if ref, w := IntegratedPullUpCredit(m, at); ref != "" {
			note := ref + " pulls the net up inside the part"
			if w != nil {
				note = w.Statement
			}
			return []goalLeg{{nets: []*ir.Net{at}, note: note}}
		}
		return nil
	}
	SortPullUpTerminations(paths)
	ix := partNets(m)
	pinOn := func(ref, net string) string {
		for _, pc := range ix.conns[ref] {
			if pc.net.Name == net {
				return pc.pin
			}
		}
		return ""
	}
	var legs []goalLeg
	for _, p := range paths {
		leg := goalLeg{nets: []*ir.Net{at}}
		from := at.Name
		for _, h := range p.Path {
			leg.crossings = append(leg.crossings, TraceCross{
				RefDes: h.Resistor, Class: string(m.ComponentClass(h.Resistor)),
				EnterPin: pinOn(h.Resistor, from), ExitPin: pinOn(h.Resistor, h.Net),
				FromNet: from, ToNet: h.Net,
			})
			leg.nets = append(leg.nets, byName[h.Net])
			from = h.Net
		}
		legs = append(legs, leg)
	}
	return legs
}
