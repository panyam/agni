package profiles

import (
	"context"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Coverage states (WS9-041). Each matches the condition a profile rule fires on, so a coverage cell
// and a finding never disagree. missing is signal-missing, dangling is signal-dangling, and
// pullup_missing is missing-pullup.
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

// InterfaceCoverage is one DETECTED interface profile's coverage. Anchor is the net the profile is
// anchored at, for context and locate, and Signals holds every required signal in profile order.
type InterfaceCoverage struct {
	Profile string
	Anchor  string
	Signals []SignalCoverage
}

// Coverage projects a profile onto a design's per-signal coverage matrix, or nil when the interface
// is not DETECTED, matching the rules. DETECTED means two of its signals are present or a component
// declares the interface via its host attribute. It uses the same signal matcher (matcher.go) and the
// same check.PullUpReachesRail the profile rules decide on, so the panel and the findings cannot
// drift; see InUse in present.go for why they must agree.
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
// component connection. That is the net component.net(?r,?n) plus netMatch(?n, s) selects, so the
// panel binds the net a finding would name and not a foreign one that merely shares a suffix.
func matchSignalNet(m check.Model, s Signal) *ir.Net {
	for _, n := range m.Nets() {
		if netMatchesSignal(n.GetName(), s) && len(n.GetConnections()) > 0 {
			return n
		}
	}
	return nil
}

// reachesRail reports whether the net reaches a power rail through a pull-up, by calling the same
// check.PullUpReachesRail the missing-pullup rule decides on (agni issue 516).
//
// Do not replace it with a `net.reaches(?n, ?rail), net.rail(?rail)` query. The reach walk refuses a net whose
// fan-out exceeds maxWalkFan (WS3-108), and a rail is nearly always that wide, so a DIRECT pull-up
// onto a real rail is invisible to it. Measured with a resistor between a signal and a 21-connection
// rail, that query said false where PullUpReachesRail said true, and the panel scored a clean bus
// `pullup_missing` while the rule stayed silent.
func reachesRail(m check.Model, net string) bool {
	for _, n := range m.Nets() {
		if n.GetName() == net {
			return check.PullUpReachesRail(m, n)
		}
	}
	return false
}

// hostDeclares reports whether a component declares this interface via its host attribute
// (interface=<name>), the WS3-042 host binding, which detects an interface independently of the
// in-use gate.
func hostDeclares(base *query.Base, p Profile) bool {
	if !p.HasHost() {
		return false
	}
	q := query.Build(p.hostRules(),
		[]query.Literal{query.Pos(query.Rel("host", query.V("ref")))}, query.V("ref"))
	rows, err := query.Default.Eval(context.Background(), q, base)
	return err == nil && len(rows) > 0
}
