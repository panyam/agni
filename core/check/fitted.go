package check

import (
	"context"
	"fmt"
)

// OnFittedParts marks r as a rule that judges a part on the assembled board, such as a part's own
// voltage rating or an LED's orientation, and returns it. A verdict that names a part marked
// do-not-populate, directly or through one of its pins, becomes not considered and says which part,
// because the assembled board has no such part to judge (agni issue 1082). It is the blame-side twin
// of a credit-side rule asking Model.IsFitted, which agni issue 961 made every crediting rule do.
//
// It is opt-in, because a rule about a part can still matter when the part is not fitted: two
// unfitted parts sharing a reference designator are still a netlist defect. It wraps r's Eval in
// place, so every surface that evaluates r gets it, and marking a rule twice wraps it once.
func OnFittedParts(r *Rule) *Rule {
	if r.onFittedParts {
		return r
	}
	r.onFittedParts = true
	eval := r.Eval
	r.Eval = func(ctx context.Context, m Model) []Verdict {
		vs := eval(ctx, m)
		for i := range vs {
			if ref := unfittedSubject(m, vs[i].Subjects); ref != "" {
				vs[i].Outcome = NotConsidered
				vs[i].Reason = fmt.Sprintf("%s is marked do-not-populate, so the assembled board has no such part to judge", ref)
				vs[i].Finding = nil
				vs[i].Witness = nil
			}
		}
		return vs
	}
	return r
}

// unfittedSubject returns the first part among subjects, named as a component or through one of its
// pins, that is marked do-not-populate, or "" when every one is fitted.
func unfittedSubject(m Model, subjects []Entity) string {
	for _, s := range subjects {
		if (s.Kind == KindComponent || s.Kind == KindPin) && !m.IsFitted(s.Ref) {
			return s.Ref
		}
	}
	return ""
}

// JudgesFittedParts reports whether OnFittedParts has marked r.
func (r *Rule) JudgesFittedParts() bool { return r.onFittedParts }
