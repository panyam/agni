package check

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// DefaultWalkHops is the radius a walk takes when the caller names none. Hop 1 is the start's own
// nets, so two hops show the parts around it and the nets just past them. A walk widens hop by hop
// (agni issue 1337), so the default is small.
const DefaultWalkHops = 2

// smallPartPins is the most connected pins a part may have for the small-parts walk to cross it.
// Four keeps a resistor, a capacitor, a dual diode and a SOT-23 FET, and leaves out every IC.
const smallPartPins = 4

// walkHopNetCap bounds one hop of the all-parts walk. One hop through a microcontroller reaches
// every net it touches, and the walk counts what it left out rather than listing them.
const walkHopNetCap = 64

// ErrUnknownWalkKind is the error for a walk kind the registry does not hold. A caller lists the
// kinds with WalkKinds.
var ErrUnknownWalkKind = errors.New("unknown walk kind")

// WalkKind names one way a walk chooses which parts to cross. Each answers a different question
// about a part, so a viewer offers them side by side (agni issue 1339).
type WalkKind struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

// walkRule is a WalkKind and its crossing rule. cross reports whether the walk crosses ref, and
// when it does not, the reason the answer gives for leaving ref at the boundary.
type walkRule struct {
	WalkKind
	netCap int
	// throughRails lets the walk pass through a rail-named net that is not bus-like, as trace does,
	// so a series walk follows a signal through a fuse on VBUS.
	throughRails bool
	// pairs makes a crossing from a paired pin (Model.ThroughPartner) exit through its partner alone,
	// and crossPairs crosses such a part whatever cross says.
	pairs, crossPairs bool
	cross             func(m Model, ref string, nets, pins int) (bool, string)
}

var walkRules = []walkRule{
	{
		WalkKind: WalkKind{
			Name:    "series",
			Title:   "Series path",
			Summary: "Crosses the two-terminal series parts agni trace crosses (resistors, inductors, ferrites, fuses, net ties), and like trace it passes through a rail-named node, so it follows a signal through them.",
		},
		throughRails: true,
		pairs:        true,
		cross: func(m Model, ref string, nets, _ int) (bool, string) {
			c := m.ComponentClass(ref)
			if passClass(c) && nets == 2 {
				return true, ""
			}
			return false, fmt.Sprintf("%s is not a series part", classLabel(c))
		},
	},
	{
		WalkKind: WalkKind{
			Name:    "small-parts",
			Title:   "Small parts",
			Summary: fmt.Sprintf("Crosses any part with up to %d connected pins, capacitors and diodes included, and an IC's paired pins such as a level translator's A2 and B2. Stops at other ICs, connectors and rails.", smallPartPins),
		},
		pairs:      true,
		crossPairs: true,
		cross: func(m Model, ref string, _, pins int) (bool, string) {
			switch c := m.ComponentClass(ref); {
			case c == ClassConnector:
				return false, "a connector"
			case pins > smallPartPins:
				return false, fmt.Sprintf("%d connected pins", pins)
			}
			return true, ""
		},
	},
	{
		WalkKind: WalkKind{
			Name:    "all",
			Title:   "Every part",
			Summary: fmt.Sprintf("Crosses every fitted part, stops at rails, and stops a hop at %d nets, counting the rest.", walkHopNetCap),
		},
		netCap: walkHopNetCap,
		cross:  func(Model, string, int, int) (bool, string) { return true, "" },
	},
}

// WalkKinds lists the walk kinds in the order a viewer offers them.
func WalkKinds() []WalkKind {
	out := make([]WalkKind, len(walkRules))
	for i, r := range walkRules {
		out[i] = r.WalkKind
	}
	return out
}

func walkRuleOf(name string) (walkRule, bool) {
	for _, r := range walkRules {
		if r.Name == name {
			return r, true
		}
	}
	return walkRule{}, false
}

// WalkNet is one net a walk reached. Entered says how it was first reached. On a hop-1 net it is the start part's own pin,
// with an empty FromNet. Ended says why the walk goes no further from this net ("ground", "rail",
// "global", or a fan-out), and is empty when the net was expanded or lies at the radius.
type WalkNet struct {
	Hop     int        `json:"hop"`
	Net     TraceNet   `json:"net"`
	Entered TraceCross `json:"entered"`
	// AlsoEntered lists the later crossings into this net, by parts that reached it after Entered
	// did, such as a crystal's second load capacitor into GND.
	AlsoEntered []TraceCross `json:"also_entered,omitempty"`
	Ended       string       `json:"ended,omitempty"`
}

// WalkBoundary is a part on a reached net that the walk did not cross, with the reason. A reader
// widens the walk past it by naming it in `through`.
type WalkBoundary struct {
	RefDes string `json:"ref_des"`
	Class  string `json:"class,omitempty"`
	Hop    int    `json:"hop"`
	Net    string `json:"net"`
	Reason string `json:"reason"`
}

// WalkElided counts the nets one hop left out under the kind's cap.
type WalkElided struct {
	Hop  int `json:"hop"`
	Nets int `json:"nets"`
}

// Walk is the answer to "what is around this part, hop by hop". Reason is set when the start named
// nothing the design has, and nothing was walked then.
type Walk struct {
	From     TraceEnd       `json:"from"`
	Kind     string         `json:"kind"`
	Radius   int            `json:"radius"`
	Reason   string         `json:"reason,omitempty"`
	Nets     []WalkNet      `json:"nets,omitempty"`
	Boundary []WalkBoundary `json:"boundary,omitempty"`
	Elided   []WalkElided   `json:"elided,omitempty"`
	// Goal is the answer to a goal question asked from the walk (WalkGoalFrom), nil when none was.
	Goal *WalkGoalAnswer `json:"goal,omitempty"`
}

// WalkFrom walks outwards from a part, or from one pin when from names a pin, hop by hop.
//
// Hop 1 is the start's own nets. Each later hop crosses the parts on the previous hop's nets that
// the kind's rule allows, or that through names, into their other nets. A ground or a global net is
// reached and never expanded, and so is a rail except on the series walk. The answer says which
// branches ended there. A part
// the rule refuses is listed in Boundary with its reason, once, at the first hop that met it. An
// unknown kind is ErrUnknownWalkKind. A start that names nothing is an answer with Reason set, as an
// unresolved trace endpoint is.
func WalkFrom(m Model, from Endpoint, kind string, hops int, through []string) (Walk, error) {
	if kind == "" {
		kind = walkRules[0].Name
	}
	rule, ok := walkRuleOf(kind)
	if !ok {
		names := make([]string, len(walkRules))
		for i, r := range walkRules {
			names[i] = r.Name
		}
		return Walk{}, fmt.Errorf("%w %q: want one of %s", ErrUnknownWalkKind, kind, strings.Join(names, ", "))
	}
	if hops <= 0 {
		hops = DefaultWalkHops
	}
	w := Walk{Kind: kind, Radius: hops, From: TraceEnd{Endpoint: from}}
	ix := partNets(m)

	// The start's nets, each entered through the start part's own pin.
	type entry struct {
		net   *ir.Net
		cross TraceCross
	}
	var starts []entry
	var ends []Endpoint
	if from.Pin != "" {
		n, end, reason := resolveEndpoint(m, from)
		w.From = end
		if reason != "" {
			w.Reason = reason
			return w, nil
		}
		ends = []Endpoint{end.Endpoint}
		starts = append(starts, entry{n, TraceCross{RefDes: end.RefDes, Class: string(m.ComponentClass(end.RefDes)), ExitPin: end.Pin, ToNet: n.Name}})
	} else {
		if from.RefDes == "" || !m.HasComponent(from.RefDes) {
			w.Reason = fmt.Sprintf("no component %s in this design", from.RefDes)
			return w, nil
		}
		seen := map[*ir.Net]bool{}
		for _, pc := range ix.conns[from.RefDes] {
			if seen[pc.net] {
				continue
			}
			seen[pc.net] = true
			starts = append(starts, entry{pc.net, TraceCross{RefDes: from.RefDes, Class: string(m.ComponentClass(from.RefDes)), ExitPin: pc.pin, ToNet: pc.net.Name}})
		}
		if len(starts) == 0 {
			w.Reason = fmt.Sprintf("%s sits on no net", from.RefDes)
			return w, nil
		}
	}

	visited := map[*ir.Net]bool{}
	crossed := map[string]bool{from.RefDes: true}
	crossedPin := map[string]bool{}
	pairedPart := map[string]bool{}
	hasPairs := func(ref string) bool {
		known, ok := pairedPart[ref]
		if !ok {
			for _, pc := range ix.conns[ref] {
				if _, p := m.ThroughPartner(ref, pc.pin); p {
					known = true
					break
				}
			}
			pairedPart[ref] = known
		}
		return known
	}
	through1 := map[string]bool{}
	for _, t := range through {
		through1[t] = true
	}
	boundary := map[string]bool{}
	route := map[*ir.Net][]string{}
	var reached []*ir.Net
	index := map[*ir.Net]int{}
	add := func(hop int, n *ir.Net, c TraceCross) bool {
		index[n] = len(w.Nets)
		visited[n] = true
		route[n] = append(route[n], c.RefDes)
		reached = append(reached, n)
		ended := walkEnd(m, n, rule.throughRails)
		w.Nets = append(w.Nets, WalkNet{Hop: hop, Entered: c, Ended: ended})
		return ended == ""
	}
	var frontier []*ir.Net
	for _, s := range starts {
		if !visited[s.net] && add(1, s.net, s.cross) {
			frontier = append(frontier, s.net)
		}
	}
	for hop := 2; hop <= hops && len(frontier) > 0; hop++ {
		var next []*ir.Net
		added, elided := 0, 0
		for _, n := range frontier {
			for _, c := range n.Connections {
				ref := c.ComponentRef
				if crossed[ref] || crossedPin[ref+"\x00"+c.PinRef] || IsVirtualRef(ref) {
					continue
				}
				pcs := ix.conns[ref]
				// A pin with a through partner (a translator's A2 and B2) carries one signal across the
				// part, so the walk follows that pin alone rather than every pin of the part.
				partner, paired := "", false
				if rule.pairs {
					partner, paired = m.ThroughPartner(ref, c.PinRef)
					// On a part with through pairs, an unpaired pin (a supply, a ground, an enable)
					// carries no signal across, so the walk crosses nothing from it.
					if !paired && hasPairs(ref) {
						continue
					}
				}
				ok, why := through1[ref], ""
				if !ok {
					switch {
					case !m.IsFitted(ref):
						why = "not fitted"
					case paired && rule.crossPairs:
						ok = true
					default:
						ok, why = rule.cross(m, ref, ix.netCount(ref), len(pcs))
					}
				}
				if !ok {
					if !boundary[ref] {
						boundary[ref] = true
						w.Boundary = append(w.Boundary, WalkBoundary{
							RefDes: ref, Class: string(m.ComponentClass(ref)), Hop: hop - 1, Net: n.Name, Reason: why,
						})
					}
					continue
				}
				route[n] = append(route[n], ref)
				if paired {
					// Only this channel is crossed, so the part's other channels stay open to the walk.
					crossedPin[ref+"\x00"+c.PinRef], crossedPin[ref+"\x00"+partner] = true, true
				} else {
					crossed[ref] = true
				}
				for _, pc := range pcs {
					if pc.net == n || (paired && pc.pin != partner) {
						continue
					}
					if visited[pc.net] {
						// Reached already, so this crossing adds no net. The net records it as a further
						// way in, and the part leaves the net's list of other parts. A net the cap left
						// out has no entry to record it on.
						route[pc.net] = append(route[pc.net], ref)
						if i, ok := index[pc.net]; ok {
							w.Nets[i].AlsoEntered = append(w.Nets[i].AlsoEntered, TraceCross{
								RefDes: ref, Class: string(m.ComponentClass(ref)),
								EnterPin: c.PinRef, ExitPin: pc.pin, FromNet: n.Name, ToNet: pc.net.Name, Paired: paired,
							})
						}
						continue
					}
					if rule.netCap > 0 && added >= rule.netCap {
						elided++
						visited[pc.net] = true // counted once, however many parts reach it
						continue
					}
					added++
					if add(hop, pc.net, TraceCross{
						RefDes: ref, Class: string(m.ComponentClass(ref)),
						EnterPin: c.PinRef, ExitPin: pc.pin, FromNet: n.Name, ToNet: pc.net.Name, Paired: paired,
					}) {
						next = append(next, pc.net)
					}
				}
			}
		}
		if elided > 0 {
			w.Elided = append(w.Elided, WalkElided{Hop: hop, Nets: elided})
		}
		frontier = next
	}
	for i, n := range reached {
		w.Nets[i].Net = traceNet(m, n, route[n], ends)
	}
	sort.SliceStable(w.Boundary, func(i, j int) bool {
		if w.Boundary[i].Hop != w.Boundary[j].Hop {
			return w.Boundary[i].Hop < w.Boundary[j].Hop
		}
		return w.Boundary[i].RefDes < w.Boundary[j].RefDes
	})
	return w, nil
}

// walkEnd says why a walk goes no further from n, or "" when it may cross on. It stops at everything
// IsBusLike stops at. Unless throughRails is set it stops at a rail too, since a walk that widened
// through a 3V3 rail would list every part the rail feeds. The series walk passes through a
// rail-named node (VBUS_PROT) as trace does, so it agrees with trace.
func walkEnd(m Model, n *ir.Net, throughRails bool) string {
	switch {
	case m.IsGroundNet(n):
		return "ground"
	case n.GetAttributes()[netgraph.AttrGlobal] == "true":
		return "global"
	case len(n.Connections) > maxWalkFan:
		return fmt.Sprintf("fan-out of %d", len(n.Connections))
	case !throughRails && m.IsRailNet(n):
		return "rail"
	}
	return ""
}

// classLabel is a component class as a reason reads it, "a capacitor" or "a part" for one with none.
func classLabel(c ComponentClass) string {
	if c == "" || c == ClassUnknown {
		return "a part with no class"
	}
	s := string(c)
	if strings.ContainsRune("aeiou", rune(s[0])) {
		return "an " + s
	}
	return "a " + s
}

// pinConn is one connected pin of a part.
type pinConn struct {
	net *ir.Net
	pin string
}

// partIndex lists each part's connected pins in netlist order.
type partIndex struct {
	conns map[string][]pinConn
}

func partNets(m Model) partIndex {
	ix := partIndex{conns: map[string][]pinConn{}}
	for _, n := range m.Nets() {
		for _, c := range n.Connections {
			ix.conns[c.ComponentRef] = append(ix.conns[c.ComponentRef], pinConn{n, c.PinRef})
		}
	}
	return ix
}

// netCount is how many distinct nets ref touches.
func (ix partIndex) netCount(ref string) int {
	seen := map[*ir.Net]bool{}
	for _, pc := range ix.conns[ref] {
		seen[pc.net] = true
	}
	return len(seen)
}
