package profiles

import (
	"strings"

	"github.com/panyam/agni/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/query"
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
// attribute. It reuses the same net-suffix matching and reaches-rail pull-up walk the profile rules
// compile to, so the panel and the findings cannot drift.
func Coverage(p Profile, m check.Model) *InterfaceCoverage {
	base := query.NewBase(m)
	nets := make([]*ir.Net, len(p.Signals))
	present := 0
	anchor := ""
	for i, s := range p.Signals {
		n := matchSignalNet(m, s.Suffix)
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
		case s.PullUp && !reachesRail(base, n.GetName()):
			sc.Net, sc.State = n.GetName(), StatePullupMissing
		default:
			sc.Net, sc.State = n.GetName(), StatePresent
		}
		cov.Signals = append(cov.Signals, sc)
	}
	return cov
}

// matchSignalNet returns the first net whose name has the given suffix and carries at least one
// component connection — the same net component-on-net(?r,?n), suffix(?n, S) selects.
func matchSignalNet(m check.Model, suffix string) *ir.Net {
	for _, n := range m.Nets() {
		if strings.HasSuffix(n.GetName(), suffix) && len(n.GetConnections()) > 0 {
			return n
		}
	}
	return nil
}

// reachesRail reports whether the net reaches a power rail through the reach walk (a pull-up path),
// the same reaches(?n, ?rail), rail(?rail) the missing-pullup rule negates.
func reachesRail(base *query.Base, net string) bool {
	q := query.Build(nil,
		[]query.Literal{
			query.Pos(query.Rel("reaches", query.Str(net), query.V("rail"))),
			query.Pos(query.Rel("rail", query.V("rail"))),
		},
		query.V("rail"))
	rows, err := query.Naive{}.Eval(q, base)
	return err == nil && len(rows) > 0
}

// hostDeclares reports whether a component declares this interface via its host attribute
// (interface=<name>), the WS3-042 host binding — an alternative detection signal to the in-use gate.
func hostDeclares(base *query.Base, p Profile) bool {
	if !p.HasHost() {
		return false
	}
	q := query.Build([]query.Rule{p.hostRule()},
		[]query.Literal{query.Pos(query.Rel("host", query.V("ref")))}, query.V("ref"))
	rows, err := query.Naive{}.Eval(q, base)
	return err == nil && len(rows) > 0
}
