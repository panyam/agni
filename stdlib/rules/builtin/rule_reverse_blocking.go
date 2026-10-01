package builtin

import (
	"context"
	"fmt"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/internal/netgraph"
)

// reverseBlockingAbsent flags a connector-fed power path with no DIRECTIONAL blocking element
// (WS3-094). It walks the same path as input-protection but asks whether anything stops current
// flowing the WRONG WAY, where that rule asks for a fuse or TVS. Why a fuse and a TVS do not count,
// and why a transistor reads as unclassifiable, are in docs/reverse-blocking-absent.md.
var reverseBlockingAbsent = &check.Rule{
	Name:       "reverse-blocking-absent",
	Severity:   "warning",
	Summary:    "A connector feeds a power input with no directional element blocking reverse flow.",
	Impact:     "Reverse polarity from a miswired connector, or backfeed from a parallel source into a switched-off rail, reaches the board unopposed. ISO 16750-2 makes reverse voltage a qualification requirement on a vehicle, and a fuse does not help: it opens on magnitude, not direction.",
	Remedy:     "Add a directional element between the connector and the load: a series FET where the voltage drop matters, a diode where it does not, or a bridge where the input polarity is genuinely unknown.",
	Primitives: []string{"select", "traverse", "reach", "pin-role"},
	Reads:      []string{"component.class", "net.attributes", "on_net", "pin.electrical_type", "pin.role"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryPower,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
	},
	Detail:              ruleDoc("reverse-blocking-absent"),
	Eval:                reverseBlockingVerdicts,
	StatesConsideredSet: true,
}

// reverseBlockingVerdicts decides every connector net that feeds a power input. It passes a
// blocked path, fails an unblocked one, and answers Inconclusive when an unidentified transistor
// is in the way (see docsite/content/build/check-rule.md#five-outcomes-and-the-three-that-are-not-a-pass).
// A connector net reaching NO power input gets no verdict, since counting it would claim every
// signal pin on every connector as reverse-protected (agni issue 391).
func reverseBlockingVerdicts(ctx context.Context, m check.Model) []check.Verdict {
	var out []check.Verdict
	for _, n := range m.Nets() {
		if n.Attributes[netgraph.AttrExternal] == "true" || m.IsGroundNet(n) {
			continue
		}
		hasConn := check.Exists(n.Connections, func(c *ir.Connection) bool {
			return m.HasClass(c.ComponentRef, check.ClassConnector)
		})
		if !hasConn {
			continue
		}
		outcome, ref := classifyPowerPath(m, n)
		if outcome == pathNoLoad {
			continue // no power input downstream, so there is no power path to block
		}

		v := check.Verdict{Subjects: []check.Entity{check.Entity{Kind: check.KindNet, Ref: n.GetName()}}}
		switch outcome {
		case pathProtected:
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("%s stands between the connector and the power input it feeds, and passes current one way", ref),
				Terms:     []check.WitnessTerm{{Label: "blocking element", Value: ref}},
			}
			v.Context = compContext(ref, "reverse-blocking element")
		case pathUnblocked:
			v.Outcome = check.Fail
			v.Witness = &check.Witness{
				Statement: "the connector reaches a power input through passives alone, so nothing in the path passes current one way",
			}
			v.Finding = &check.Finding{Subject: check.Entity{Kind: check.KindNet, Ref: n.GetName()}, Prov: n.GetProv(), Message: "connector feeds a power input with no reverse-blocking element in the path"}
		case pathUnclassifiable:
			// Inconclusive, not NotConsidered, because the rule reached the comparison and a netlist
			// cannot decide it. It still carries a finding so a reviewer sees it (agni issue 74).
			v.Outcome = check.Inconclusive
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("transistor %s is in the path and a netlist states nothing that separates an ideal-diode controller from an ordinary switch", ref),
				Terms:     []check.WitnessTerm{{Label: "unidentified transistor", Value: ref}},
			}
			v.Context = compContext(ref, "transistor")
			v.Finding = &check.Finding{
				Subject: check.Entity{Kind: check.KindNet, Ref: n.GetName()}, Prov: n.GetProv(), Inconclusive: true,
				Message: fmt.Sprintf(
					"connector feeds a power input through transistor %s, which may be an ideal diode or "+
						"ORing FET providing reverse protection, or may be an ordinary switch providing none. "+
						"A netlist cannot tell them apart. Seed %s's datasheet with a device_class of "+
						"ideal_diode_controller (or confirm by hand that reverse flow is blocked).", ref, ref),
				// The transistor the reader has to identify. The subject is the net, so without
				// this the part is named only in prose (agni issue 349).
				Context: compContext(ref, "transistor"),
			}
		}
		out = append(out, v)
	}
	return out
}

// classifyPowerPath decides what n's power path does about reverse flow.
//
// check.Reach crosses only two-terminal PASSIVES (resistor, inductor, ferrite, fuse), never a diode
// or a transistor. So a power input reachable through the walk means NOTHING directional stands
// between the connector and the load. A directional part stops the walk, and the rule then has to
// inspect what stopped it, because a backwards diode stops the walk exactly as a correct one does.
func classifyPowerPath(m check.Model, n *ir.Net) (pathVerdict, string) {
	r := m.Reach(n, check.PowerPathReachHops)
	inReach := map[string]bool{}
	for _, rn := range r.Nets {
		inReach[rn.GetName()] = true
		if hasPowerInput(m, rn) {
			return pathUnblocked, "" // reached a load through passives alone, so nothing directional is in the way
		}
	}
	// Nothing reachable, so something stopped the walk.
	//
	// A TRANSISTOR anywhere on the neighborhood settles it. It is checked here and not in the bridging
	// loop below because farNet returns nil for a 3-terminal MOSFET, which touches two nets outside
	// the reach set (agni issue 63, 14 false FAILs on a real board).
	//
	// A datasheet-identified ideal diode, ORing or power-mux controller is checked FIRST, so a design
	// carrying both the controller and its FET reads as protected rather than unclassifiable.
	var transistor string
	for _, rn := range r.Nets {
		for _, c := range rn.GetConnections() {
			ref := c.GetComponentRef()
			if m.HasClass(ref, check.ClassIdealDiodeController) {
				return pathProtected, ref
			}
			if transistor == "" && m.ComponentClass(ref) == check.ClassTransistor {
				transistor = ref
			}
		}
	}
	if transistor != "" {
		// Structure cannot tell an ideal diode from an ordinary switch. Silence here would read as
		// a pass to a bound review item (agni issue 74).
		return pathUnclassifiable, transistor
	}
	// blocker is the directional part the walk stopped at. Without one the path is pathNoLoad, not
	// pathProtected, since a pass needs a blocking part between connector and load (agni issue 391).
	blocker := ""
	unblocked := false
	for _, rn := range r.Nets {
		for _, c := range rn.GetConnections() {
			ref := c.GetComponentRef()
			far := farNet(m, ref, inReach)
			// A part whose far terminal lands on GROUND is a shunt beside the path, so it says
			// nothing about reverse blocking. A freewheel diode across an inductive load otherwise
			// fails the orientation test below (issue 63, 20 false FAILs). Ground only, because a
			// series blocking diode's far side is often a NAMED RAIL (connector -> D1 -> +12V_SW ->
			// regulator), so excluding rails would silence the rule.
			if far == nil || m.IsGroundNet(far) || !feedsPowerInput(m, far) {
				continue
			}
			if m.ComponentClass(ref) == check.ClassDiode {
				if pinNetWithRole(m, ref, check.RoleAnode) != rn.GetName() {
					unblocked = true // fitted backwards, so it blocks the supply and not the fault
				} else if blocker == "" {
					blocker = ref // fitted the right way round, so it is the path's blocking element
				}
			}
		}
	}
	switch {
	case unblocked:
		return pathUnblocked, ""
	case blocker != "":
		return pathProtected, blocker
	}
	return pathNoLoad, ""
}

// pathVerdict is what the walk concluded about one connector-fed net. "Verified protected" and
// "could not tell" are separate answers (agni issue 74).
type pathVerdict int

const (
	// pathProtected: a directional element stands between the connector and the load, either a
	// correctly-fitted diode or a datasheet-identified ideal-diode controller. A genuine pass.
	pathProtected pathVerdict = iota
	// pathUnblocked: nothing directional is in the way, or a diode is fitted backwards. The defect.
	pathUnblocked
	// pathUnclassifiable: a transistor is on the path and nothing identifies it. Reported as an
	// INCONCLUSIVE finding, never as a defect.
	pathUnclassifiable
	// pathNoLoad: the connector net reaches no power input, in its passive neighborhood or across a
	// part bridging out of it. The net is not a power path, so it gets no verdict (agni issue 391).
	pathNoLoad
)

// hasPowerInput reports whether a net carries a real power-input pin (virtual power symbols excluded).
func hasPowerInput(m check.Model, n *ir.Net) bool {
	return check.Exists(n.GetConnections(), func(c *ir.Connection) bool {
		return !check.IsVirtualRef(c.GetComponentRef()) && check.ConnDir(m, c) == ir.PinDirection_PIN_DIRECTION_POWER_IN
	})
}

// feedsPowerInput reports whether a power input sits on n or in its passive neighborhood.
func feedsPowerInput(m check.Model, n *ir.Net) bool {
	for _, rn := range m.Reach(n, check.PowerPathReachHops).Nets {
		if hasPowerInput(m, rn) {
			return true
		}
	}
	return false
}

// farNet returns the single net ref touches OUTSIDE the given set, or nil when it touches none or
// several. A two-terminal series part has exactly one far side, and this rule does not reason about
// anything else.
func farNet(m check.Model, ref string, inReach map[string]bool) *ir.Net {
	var out *ir.Net
	for _, n := range m.Nets() {
		if inReach[n.GetName()] || !touchesRef(n, ref) {
			continue
		}
		if out != nil {
			return nil
		}
		out = n
	}
	return out
}

// touchesRef reports whether refDes has a connection on n.
func touchesRef(n *ir.Net, refDes string) bool {
	return check.Exists(n.GetConnections(), func(c *ir.Connection) bool {
		return c.GetComponentRef() == refDes
	})
}

// pinNetWithRole returns the net name carrying ref's pin of the given role, or "" when the part
// declares no such pin. A diode whose part type names no anode yields "", and the caller treats
// unknown orientation as backwards.
func pinNetWithRole(m check.Model, ref string, role check.PinRole) string {
	for _, p := range m.Pins() {
		if p.Component.GetRefDes() != ref {
			continue
		}
		if m.PinRole(ref, p.Designator) == role {
			return m.PinNetName(ref, p.Designator)
		}
	}
	return ""
}
