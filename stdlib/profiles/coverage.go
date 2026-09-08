package profiles

import (
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Coverage states (WS9-041). They match the conditions the profile rules fire on, so a coverage
// cell and a finding never disagree: missing == signal-missing, dangling == signal-dangling,
// pullup_missing == missing-pullup.
const (
	StatePresent       = "present"
	StateMissing       = "missing"
	StateDangling      = "dangling"
	StatePullupMissing = "pullup_missing"
)

// SignalCoverage is one required signal's state within a detected interface. Net is the matched net
// name, "" when the signal is Missing.
type SignalCoverage struct {
	Name  string
	Net   string
	State string
}

// InterfaceCoverage is one DETECTED interface profile's coverage: the profile name, the net it is
// anchored at (for context/locate), and every required signal in profile order.
type InterfaceCoverage struct {
	Profile string
	Anchor  string
	Signals []SignalCoverage
}

// Coverage projects a profile onto a design's per-signal coverage matrix, or nil when the interface
// is not DETECTED — silent by construction, matching the rules. Detection is the profile's in-use
// confidence gate: two of its signals present, or a component declares the interface via its host
// attribute. It reuses the same signal matcher (matcher.go) and reaches-rail pull-up walk the profile
// rules compile to, so the panel and the findings cannot drift. The pull-up half calls
// check.PullUpReachesRail, which is the SAME function the missing-pullup rule decides on, so "cannot
// drift" is now true by construction rather than by two implementations agreeing.
func Coverage(p Profile, m check.Model) *InterfaceCoverage {
	base := query.NewBase(m)
	nets := make([]*ir.Net, len(p.Signals))
	present := 0
	anchor := ""
	for i, s := range p.Signals {
		n := matchSignalNet(m, s)
		nets[i] = n
		if n != nil {
			present++
			if s.Anchor {
				anchor = n.GetName()
			}
		}
	}
	if present < 2 && !hostDeclares(base, p) {
		return nil
	}
	cov := &InterfaceCoverage{Profile: p.Name, Anchor: anchor}
	for i, s := range p.Signals {
		sc := SignalCoverage{Name: s.Name}
		switch n := nets[i]; {
		case n == nil:
			sc.State = StateMissing
		case len(n.GetConnections()) < 2:
			sc.Net, sc.State = n.GetName(), StateDangling
		case s.PullUp && !reachesRail(m, n.GetName()):
			sc.Net, sc.State = n.GetName(), StatePullupMissing
		default:
			sc.Net, sc.State = n.GetName(), StatePresent
		}
		cov.Signals = append(cov.Signals, sc)
	}
	return cov
}

// matchSignalNet returns the first net satisfying the signal's matcher that carries at least one
// component connection — the same net component-on-net(?r,?n) plus netMatch(?n, s) selects, so the
// coverage panel binds the net a finding would name and not a foreign one that merely shares a suffix.
func matchSignalNet(m check.Model, s Signal) *ir.Net {
	for _, n := range m.Nets() {
		if netMatchesSignal(n.GetName(), s) && len(n.GetConnections()) > 0 {
			return n
		}
	}
	return nil
}

// reachesRail reports whether the net reaches a power rail through a pull-up, by calling the same
// check.PullUpReachesRail the missing-pullup rule decides on.
//
// IT USED TO ASK ITS OWN QUESTION, and the two disagreed on the common case. This built a
// `reaches(?n, ?rail), rail(?rail)` query and claimed in a comment that it was what the rule negated;
// the rule had a second clause the comment did not mention, and that clause existed because the reach
// walk refuses to enter a net whose fan-out exceeds maxWalkFan (WS3-108). A rail is wide almost by
// definition, so a DIRECT pull-up onto a real rail was invisible to the reaches form. Measured on a
// resistor sitting on a signal and on a 21-connection rail: this returned false while
// PullUpReachesRail returned true, so the coverage panel scored a correctly pulled signal
// `pullup_missing` and the rule, correctly, said nothing.
//
// The panel and the rule agreeing is not a nicety. InUse's own contract says the gate must agree with
// the rules or an interface the rules will not fire on gets scored as a clean pass, and this was the
// same failure pointing the other way: a clean bus scored as a defect, in the one surface a reviewer
// reads before the findings.
func reachesRail(m check.Model, net string) bool {
	for _, n := range m.Nets() {
		if n.GetName() == net {
			return check.PullUpReachesRail(m, n)
		}
	}
	return false
}

// hostDeclares reports whether a component declares this interface via its host attribute
// (interface=<name>), the WS3-042 host binding — an alternative detection signal to the in-use gate.
func hostDeclares(base *query.Base, p Profile) bool {
	if !p.HasHost() {
		return false
	}
	q := query.Build(p.hostRules(),
		[]query.Literal{query.Pos(query.Rel("host", query.V("ref")))}, query.V("ref"))
	rows, err := query.Naive{}.Eval(q, base)
	return err == nil && len(rows) > 0
}
