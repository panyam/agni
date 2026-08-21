package builtin

import (
	"fmt"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/internal/netgraph"
)

// reverseBlockingAbsent flags a connector-fed power path with no DIRECTIONAL blocking element
// (WS3-094). Same walk as input-protection, different question: that rule asks whether a fuse or TVS
// guards the path, this asks whether anything stops current flowing the WRONG WAY. Why a fuse and a
// TVS do not count, and why a transistor reads as unclassifiable rather than unprotected, are in
// docs/reverse-blocking-absent.md.
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

// reverseBlockingVerdicts decides every connector net that feeds a power input, and it is the rule
// the Inconclusive outcome was added for. `classifyPowerPath` already answered in three values —
// protected, unblocked, and "a transistor is in the way and a netlist cannot tell an ideal-diode
// controller from an ordinary switch" — and two of the three reached a caller. The pass did not, so
// a board where every entry is correctly blocked and a board where the rule saw no power path at all
// produced the same nothing.
//
// THE FOURTH ANSWER IS NEW AND IS NOT AN OUTCOME. A connector net that reaches no power input is not
// a power path, so it gets no verdict. The old shape could not tell that apart from a protected one,
// because both took the same `return pathProtected` out of the walk, and under FailuresOnly the
// distinction cost nothing. Under a considered set it would have claimed every signal pin on every
// connector as reverse-protected.
func reverseBlockingVerdicts(m check.Model) []check.Verdict {
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

		v := check.Verdict{Kind: check.KindNet, Subject: n.GetName()}
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
			v.Finding = &check.Finding{
				Kind: check.KindNet, Subject: n.GetName(), Prov: n.GetProv(),
				Message: "connector feeds a power input with no reverse-blocking element in the path",
			}
		case pathUnclassifiable:
			// Inconclusive, not NotConsidered: the rule had everything it needed and REACHED the
			// comparison, and the discrimination itself is impossible from a netlist. It still has to
			// reach a reviewer, which is why the projection carries it to a finding where a
			// NotConsidered would stop here (agni issue 74).
			v.Outcome = check.Inconclusive
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("transistor %s is in the path and a netlist states nothing that separates an ideal-diode controller from an ordinary switch", ref),
				Terms:     []check.WitnessTerm{{Label: "unidentified transistor", Value: ref}},
			}
			v.Context = compContext(ref, "transistor")
			v.Finding = &check.Finding{
				Kind: check.KindNet, Subject: n.GetName(), Prov: n.GetProv(), Inconclusive: true,
				Message: fmt.Sprintf(
					"connector feeds a power input through transistor %s, which may be an ideal diode or "+
						"ORing FET providing reverse protection, or may be an ordinary switch providing none. "+
						"A netlist cannot tell them apart. Seed %s's datasheet with a device_class of "+
						"ideal_diode_controller (or confirm by hand that reverse flow is blocked).", ref, ref),
				// The transistor the reader has to go and identify. The subject is the net, and
				// this finding's whole remedy is about that part, so naming it only in prose made
				// the next step a manual search (agni issue 349).
				Context: compContext(ref, "transistor"),
			}
		}
		out = append(out, v)
	}
	return out
}

// classifyPowerPath decides what n's power path does about reverse flow.
//
// THE WALK IS THE MECHANISM, and it works because of what it refuses to cross. check.Reach crosses
// only two-terminal PASSIVES (resistor, inductor, ferrite, fuse). A diode is not a pass element,
// "polarity, not a wire", and neither is a transistor. So:
//
//   - A power input reachable through the passive walk means NOTHING directional stands between the
//     connector and the load. That is the finding.
//   - A directional part stops the walk, so the rule has to look at what stopped it rather than
//     conclude from silence: a backwards diode stops the walk exactly as a correct one does.
func classifyPowerPath(m check.Model, n *ir.Net) (pathVerdict, string) {
	r := m.Reach(n, check.PowerPathReachHops)
	inReach := map[string]bool{}
	for _, rn := range r.Nets {
		inReach[rn.GetName()] = true
		if hasPowerInput(m, rn) {
			return pathUnblocked, "" // reached a load through passives alone: nothing directional in the way
		}
	}
	// Nothing reachable, so something stopped the walk. Classify each part bridging out of the
	// neighborhood toward a power input.
	//
	// A TRANSISTOR anywhere on the neighborhood settles it, and is checked here rather than inside the
	// bridging loop below because that loop reaches a part only through farNet, which returns nil for
	// anything touching more than one net outside the reach set. A real 3-terminal MOSFET touches two,
	// so the guard never fired for any actual FET and a diode on the same node drove the finding
	// instead (agni issue 63: 14 false FAILs on a real board).
	//
	// A datasheet-identified ideal diode / ORing / power-mux controller is checked FIRST, because a
	// design carrying both the controller and its FET must read as protected rather than as
	// unclassifiable.
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
		// Cannot tell an ideal diode from an ordinary switch by structure, so SAY so rather than stay
		// quiet, which a bound review item reads as a pass (agni issue 74).
		return pathUnclassifiable, transistor
	}
	// blocker is the directional part the walk stopped at, and its absence is what separates a
	// protected path from NO PATH. Both used to fall out of this loop as pathProtected, because
	// nothing downstream needed the difference; a considered set does, since "there is a diode
	// between the connector and the load" and "this connector feeds no load" are not the same claim
	// and only the first is a pass.
	blocker := ""
	unblocked := false
	for _, rn := range r.Nets {
		for _, c := range rn.GetConnections() {
			ref := c.GetComponentRef()
			far := farNet(m, ref, inReach)
			// A part whose far terminal lands on GROUND is a shunt beside the path, not a series
			// element in it, so it says nothing about reverse blocking. Without this, a freewheel
			// diode across an inductive load (anode on ground, cathode on the switched output) failed
			// the orientation test below and reported the output unblocked (issue 63: 20 false
			// FAILs). Ground only, deliberately: a series blocking diode's far side is very often a
			// NAMED RAIL (connector -> D1 -> +12V_SW -> regulator), so excluding rails too would
			// silence the detection this rule exists for.
			if far == nil || m.IsGroundNet(far) || !feedsPowerInput(m, far) {
				continue
			}
			if m.ComponentClass(ref) == check.ClassDiode {
				if pinNetWithRole(m, ref, check.RoleAnode) != rn.GetName() {
					unblocked = true // fitted backwards: it blocks the supply, not the fault
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

// pathVerdict is what the walk concluded about one connector-fed net. THREE outcomes, not two:
// "verified protected" and "could not tell" are separate answers (agni issue 74).
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
	// part bridging out of it. Not an outcome at all — the net is not a power path, so it is not a
	// subject of this rule and gets no verdict (agni issue 391).
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
// several. A two-terminal series part has exactly one far side; anything more is not a simple series
// element and this rule does not reason about it.
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
