package check

import (
	"context"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Scope is the set of design entities a run is asked about, so a question about one net or part
// answers for that net or part without running each rule over the whole board (agni issue 897).
//
// It travels on the run's context (WithSubjectScope), as the work budget does, because it narrows one
// request and changes no rule's meaning. A run under a scope keeps the findings and verdicts the
// scope covers, whatever the rule did, so a rule that ignores the scope is slower and never wrong.
// A rule that declares ScopeSubjects reads it (ScopedNets, ScopedComponents, ScopedPins) and
// evaluates only those subjects, and TestScopedRunMatchesFilteredFullRun holds that to the answer a
// whole run filtered to the same subjects gives.
type Scope struct {
	nets  []Entity
	comps map[string]bool
	pins  map[string]bool // keyed on EntityRef, "U1.3"
}

// NewScope builds a scope over the given entities. A net matches by NetID when both it and the
// scope's entry carry one, and by name otherwise. A component covers itself and every pin on it, and
// a pin covers that one pin. Any other kind names nothing a rule enumerates and is ignored, so a
// scope built from bus or endpoint entities alone is empty.
func NewScope(es ...Entity) *Scope {
	s := &Scope{comps: map[string]bool{}, pins: map[string]bool{}}
	for _, e := range es {
		switch e.Kind {
		case KindNet:
			s.nets = append(s.nets, e)
		case KindComponent:
			s.comps[e.Ref] = true
		case KindPin:
			s.pins[EntityRef(e)] = true
		}
	}
	return s
}

type scopeKey struct{}

// WithSubjectScope returns ctx carrying s. A nil scope means the whole design.
func WithSubjectScope(ctx context.Context, s *Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, s)
}

// SubjectScope returns the scope ctx carries, nil for a run over the whole design.
func SubjectScope(ctx context.Context) *Scope {
	s, _ := ctx.Value(scopeKey{}).(*Scope)
	return s
}

// Covers reports whether e is one of the scope's entities, or a pin on one of its components.
func (s *Scope) Covers(e Entity) bool {
	switch e.Kind {
	case KindNet:
		for _, n := range s.nets {
			if n.NetID != "" && e.NetID != "" {
				if n.NetID == e.NetID {
					return true
				}
			} else if n.Ref == e.Ref {
				return true
			}
		}
	case KindComponent:
		return s.comps[e.Ref]
	case KindPin:
		return s.comps[e.Ref] || s.pins[EntityRef(e)]
	}
	return false
}

// coversAny reports whether a result about these subjects belongs in a scoped answer. One covered
// element is enough, so copper-clearance's (net A, net B) answers for either net. A result naming no
// net, component or pin at all is about the RUN rather than a subject (the unresolved-symbol gate's
// inconclusive, a query that failed to evaluate), and it stays, since it holds for every subject.
func (s *Scope) coversAny(es []Entity) bool {
	scopable := false
	for _, e := range es {
		switch e.Kind {
		case KindNet, KindComponent, KindPin:
			scopable = true
			if s.Covers(e) {
				return true
			}
		}
	}
	return !scopable
}

// Results narrows one run's findings and verdicts to the scope, keeping each list's order, for a
// caller holding a whole run (the service's kept results). A verdict stays when a subject is covered,
// and a finding stays when its own subject is covered or it was projected from a verdict that stays,
// which is the answer a scoped run gives. A nil scope keeps everything.
func (s *Scope) Results(fs []Finding, vs []Verdict) ([]Finding, []Verdict) {
	if s == nil {
		return fs, vs
	}
	vs = s.verdicts(vs)
	kept := make(map[string]bool, len(vs))
	for _, v := range vs {
		kept[VerdictID(v)] = true
	}
	var out []Finding
	for _, f := range fs {
		keep := s.coversAny([]Entity{f.Subject})
		for _, id := range f.VerdictIDs {
			keep = keep || kept[id]
		}
		if keep {
			out = append(out, f)
		}
	}
	return out, vs
}

// verdicts keeps the verdicts with a subject the scope covers, in their order.
func (s *Scope) verdicts(vs []Verdict) []Verdict {
	var out []Verdict
	for _, v := range vs {
		if s.coversAny(v.Subjects) {
			out = append(out, v)
		}
	}
	return out
}

// NetNames returns the names of the scope's nets, for a rule that binds them into a query.
func (s *Scope) NetNames() []string {
	out := make([]string, 0, len(s.nets))
	for _, n := range s.nets {
		out = append(out, n.Ref)
	}
	return out
}

// RefDes returns the reference designators the scope reaches, its components and the parts its pins
// sit on, for a rule that binds them into a query.
func (s *Scope) RefDes() []string {
	seen := map[string]bool{}
	var out []string
	for r := range s.comps {
		seen[r] = true
		out = append(out, r)
	}
	for p := range s.pins {
		if i := strings.LastIndex(p, "."); i >= 0 && !seen[p[:i]] {
			seen[p[:i]] = true
			out = append(out, p[:i])
		}
	}
	return out
}

// PinDesignators returns the designators of the scope's pins, for a pin rule that binds them beside
// the parts. ok is false when the scope also holds a component, since that reaches every pin of the
// part whatever its designator, and binding the designators would leave those out.
func (s *Scope) PinDesignators() (pins []string, ok bool) {
	if len(s.comps) > 0 {
		return nil, false
	}
	seen := map[string]bool{}
	for p := range s.pins {
		if i := strings.LastIndex(p, "."); i >= 0 && !seen[p[i+1:]] {
			seen[p[i+1:]] = true
			pins = append(pins, p[i+1:])
		}
	}
	return pins, true
}

// ScopedNets is m.Nets() narrowed to the scope ctx carries, every net when it carries none.
func ScopedNets(ctx context.Context, m Model) []*ir.Net {
	s := SubjectScope(ctx)
	if s == nil {
		return m.Nets()
	}
	var out []*ir.Net
	for _, n := range m.Nets() {
		if s.Covers(NetEntity(n)) {
			out = append(out, n)
		}
	}
	return out
}

// ScopedComponents is m.Components() narrowed to the scope ctx carries. A pin in the scope does not
// bring its component in, since a verdict about the part is not a verdict about one of its pins.
func ScopedComponents(ctx context.Context, m Model) []*ir.Component {
	s := SubjectScope(ctx)
	if s == nil {
		return m.Components()
	}
	var out []*ir.Component
	for _, c := range m.Components() {
		if s.Covers(ComponentEntity(c.GetRefDes())) {
			out = append(out, c)
		}
	}
	return out
}

// ScopedPins is m.Pins() narrowed to the scope ctx carries: the scope's pins and every pin of its
// components.
func ScopedPins(ctx context.Context, m Model) []PinInst {
	s := SubjectScope(ctx)
	if s == nil {
		return m.Pins()
	}
	var out []PinInst
	for _, p := range m.Pins() {
		if s.Covers(PinEntity(p.Component.GetRefDes(), p.Designator)) {
			out = append(out, p)
		}
	}
	return out
}

// Scoping is how a rule answers under a Scope.
type Scoping int

const (
	// ScopeFiltered rules evaluate the whole design and the run keeps what the scope covers. It is
	// the default, and always correct for a rule whose results name the subjects they are about.
	ScopeFiltered Scoping = iota
	// ScopeSubjects rules read the scope and evaluate only its subjects, which is where a scoped run
	// saves time. Declaring it promises the answer a filtered whole run gives.
	ScopeSubjects
	// ScopeDesignWide rules conclude about something other than a net, part or pin (a symbol file, a
	// drawn bus, a wire end), so no subset of nets or parts has an answer of its own. A scoped run
	// does not run them, and the service names them as skipped with DesignWideReason rather than
	// filtering their results down to nothing, which would read as a clean answer.
	ScopeDesignWide
)

// DesignWideReason is the skipped-rule reason a scoped run gives for a ScopeDesignWide rule.
const DesignWideReason = "concludes about the design as a whole, so it is not asked of a net or part"

// SplitDesignWide separates the rules a run under s evaluates from the design-wide rules it leaves
// out. A nil scope leaves nothing out.
func SplitDesignWide(s *Scope, rules []*Rule) (run, designWide []*Rule) {
	if s == nil {
		return rules, nil
	}
	for _, r := range rules {
		if r.Scoping == ScopeDesignWide {
			designWide = append(designWide, r)
		} else {
			run = append(run, r)
		}
	}
	return run, designWide
}
