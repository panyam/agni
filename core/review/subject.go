package review

import (
	"sort"
	"strings"

	"github.com/panyam/agni/core/check"
)

// Entity-first projection, meaning what one report says about one entity or a set of them.
//
// This is a FILTER over an existing report, never a scoped re-run. A scoped run resolves its own
// config and can disagree with the report beside it, and it redoes net solving per subject. Filtering
// makes "the union of every entity's view is the whole report" true by construction. See
// docsite/content/architecture/web-picking.md#what-is-already-known-about-a-selection.
//
// A set is the primitive and the one-entity call is a wrapper, because the useful selections (a
// sheet, a netclass, a diff's changed entities) are sets already.

// Subject identifies one entity a finding can be about, using the same (Kind, Subject) pair
// check.Finding carries.
//
// Pin is the pin designator and is only meaningful when Kind is check.KindPin. A Subject with an
// empty Pin matches EVERY pin-kind finding on that component, so a caller can ask about a part
// without enumerating its terminals.
type Subject struct {
	Kind    string
	Subject string
	Pin     string
}

// matches reports whether a finding is about this subject. Kind must agree, and a Subject with no
// Pin is deliberately broad (see the type comment).
func (s Subject) matches(f check.Finding) bool {
	if !strings.EqualFold(f.Subject.Kind, s.Kind) || !strings.EqualFold(f.Subject.Ref, s.Subject) {
		return false
	}
	return s.Pin == "" || strings.EqualFold(f.Subject.Pin, s.Pin)
}

// EntityView is what one report says about one entity, meaning the items that examined it and what
// became of them.
//
// It carries ITEMS rather than bare findings because "this item could not run at all" is also an
// answer about the entity and has no findings. A findings-only view would collapse a clean entity
// and one nothing examined into the same state.
type EntityView struct {
	Subject Subject
	// Items that produced a finding about this subject, each carrying ONLY that subject's findings.
	// The outcome is left as the run decided it, so an item that failed on twelve nets still reads
	// failed here.
	Items []ItemResult
	// Blocked lists items that never reached a verdict (needs-data and its siblings). They have no
	// findings, and they are often the most actionable thing about an entity.
	Blocked []ItemResult
	// Unmet is every datasheet fact the blocked items named, deduplicated across them.
	Unmet []check.UnmetDependency
}

// blockedOutcome reports whether an outcome means the item never reached a verdict. Anything else
// that needs "blocked" should call this rather than enumerate the outcomes again.
func blockedOutcome(o Outcome) bool {
	switch o {
	case NeedsData, NeedsDesignIntent, NotAutomated, Inconclusive:
		return true
	}
	return false
}

// ForSubjects projects a report onto a set of entities, one view per subject in order.
//
// An entity no item mentions yields an empty EntityView rather than being omitted, because "nothing
// examined this" is an answer and silence must never read as coverage.
//
// Blocked items are attached to EVERY requested subject. A needs-data item did not evaluate, so it
// has no subject, and it could not answer for any entity including this one.
func ForSubjects(r Report, subs []Subject) []EntityView {
	out := make([]EntityView, 0, len(subs))
	var blocked []ItemResult
	for _, ar := range r.Areas {
		for _, it := range ar.Items {
			if blockedOutcome(it.Outcome) {
				blocked = append(blocked, it)
			}
		}
	}
	for _, s := range subs {
		v := EntityView{Subject: s, Blocked: blocked}
		for _, ar := range r.Areas {
			for _, it := range ar.Items {
				var mine []check.Finding
				for _, f := range it.Findings {
					if s.matches(f) {
						mine = append(mine, f)
					}
				}
				if len(mine) == 0 {
					continue
				}
				scoped := it
				scoped.Findings = mine
				v.Items = append(v.Items, scoped)
			}
		}
		v.Unmet = dedupeUnmet(blocked)
		out = append(out, v)
	}
	return out
}

// ForSubject is the one-entity case of ForSubjects. It always returns a view, even for an entity
// nothing examined.
func ForSubject(r Report, s Subject) EntityView {
	return ForSubjects(r, []Subject{s})[0]
}

// dedupeUnmet collapses the unmet dependencies of several items into the set of facts to find, each
// once (keyed case-insensitively on MPN and symbol), sorted so a rendered view is stable.
func dedupeUnmet(items []ItemResult) []check.UnmetDependency {
	seen := map[string]bool{}
	var out []check.UnmetDependency
	for _, it := range items {
		for _, d := range it.Unmet {
			key := strings.ToUpper(d.MPN) + "\x00" + strings.ToUpper(d.Symbol)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MPN != out[j].MPN {
			return out[i].MPN < out[j].MPN
		}
		return out[i].Symbol < out[j].Symbol
	})
	return out
}

// SubjectsOf enumerates every entity the report has a finding about, sorted by kind, ref and pin.
//
// This is NOT the set of entities on the design, since an entity no rule examined appears nowhere
// here. That is why an entity view cannot substitute for a review pass.
func SubjectsOf(r Report) []Subject {
	seen := map[Subject]bool{}
	var out []Subject
	for _, ar := range r.Areas {
		for _, it := range ar.Items {
			for _, f := range it.Findings {
				s := Subject{Kind: f.Subject.Kind, Subject: f.Subject.Ref, Pin: f.Subject.Pin}
				if seen[s] {
					continue
				}
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].Subject != out[j].Subject {
			return out[i].Subject < out[j].Subject
		}
		return out[i].Pin < out[j].Pin
	})
	return out
}
