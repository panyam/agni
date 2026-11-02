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
//
// base supplies the fact base the host binding is asked of, and is called only for an interface with
// fewer than two signals present. Pass the one the model keeps (service.BuildModelCached), since
// projecting a base costs about 50ms on a large board and a caller walks every profile (agni issue
// 985). A nil base builds one from the default registry on each call.
//
// It is the first of Coverages, which is the one to call for an instance-keyed profile, since such a
// profile can have several buses on one design.
func Coverage(ctx context.Context, p Profile, m check.Model, base func() *query.Base) *InterfaceCoverage {
	if all := Coverages(ctx, p, m, base); len(all) > 0 {
		return all[0]
	}
	return nil
}

// Coverages is Coverage for every bus of the interface on the design, one matrix per bus in use with
// that bus's anchor net, in net order (agni issue 1246). A profile with no instance key has at most
// one. An instance-keyed profile with no bus in use but a declared host gets the design-wide matrix,
// since the host path judges the host's own nets whatever their names.
func Coverages(ctx context.Context, p Profile, m check.Model, base func() *query.Base) []*InterfaceCoverage {
	if base == nil {
		base = func() *query.Base { return query.NewBase(m) }
	}
	if p.Instance != "" {
		var out []*InterfaceCoverage
		for _, b := range p.busesInUse(m) {
			nets := make([]*ir.Net, len(p.Signals))
			for i, s := range p.Signals {
				if on := b.signals[s.Name]; len(on) > 0 {
					nets[i] = on[0]
				}
			}
			out = append(out, coverageOf(m, p, b.anchor.GetName(), nets))
		}
		if len(out) > 0 || !hostDeclares(ctx, base, p) {
			return out
		}
	}
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
	if present < 2 && !hostDeclares(ctx, base, p) {
		return nil
	}
	return []*InterfaceCoverage{coverageOf(m, p, anchor, nets)}
}

// coverageOf states each of p's signals given the net matched for it, nil when it has none.
func coverageOf(m check.Model, p Profile, anchor string, nets []*ir.Net) *InterfaceCoverage {
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
func hostDeclares(ctx context.Context, base func() *query.Base, p Profile) bool {
	if !p.HasHost() {
		return false
	}
	q := query.Build(p.hostRules(),
		[]query.Literal{query.Pos(query.Rel("host", query.V("ref")))}, query.V("ref"))
	rows, err := query.Default.Eval(ctx, q, base(), query.EvalOptions(ctx)...)
	return err == nil && len(rows) > 0
}
