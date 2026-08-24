package review

import (
	"sort"
	"strings"

	"github.com/panyam/agni/core/check"
)

// Entity-first projection: what one report says about one entity, or a set of them.
//
// This is a FILTER over a report that already exists, never a scoped re-run, and the distinction is
// load-bearing rather than an optimisation. A scoped run resolves its own config and can therefore
// disagree with the report it sits beside; it also redoes net solving, reach walks and parameter
// resolution per subject. Filtering makes "the union of every entity's view is the whole report"
// true by construction, which is the property an entity-first UI rests on.
//
// A single entity is the degenerate case of a selection, so Selection is the primitive and the
// one-entity call is a wrapper. The selections that matter are already meaningful elsewhere: a
// sheet, a netclass, the entities a semantic diff reports as changed, or the subjects affected by an
// edit.

// Subject identifies one entity a finding can be about: the same (Kind, Subject) pair check.Finding
// carries, so a caller does not have to invent a second identity scheme to ask about one.
//
// Pin is the pin designator and is only meaningful when Kind is check.KindPin. Matching a Subject
// with an empty Pin against a pin-kind finding matches EVERY pin on that component, which is what
// lets a caller ask about a part without enumerating its terminals.
type Subject struct {
	Kind    string
	Subject string
	Pin     string
}

// matches reports whether a finding is about this subject. Kind must agree; a Subject with no Pin is
// deliberately broad (see the type comment).
func (s Subject) matches(f check.Finding) bool {
	if !strings.EqualFold(f.Subject.Kind, s.Kind) || !strings.EqualFold(f.Subject.Ref, s.Subject) {
		return false
	}
	return s.Pin == "" || strings.EqualFold(f.Subject.Pin, s.Pin)
}

// EntityView is what one report says about one entity: the items that examined it and what became of
// them.
//
// It carries the ITEMS rather than bare findings because an outcome is the unit a reviewer reasons
// in: "this item failed on this pin" and "this item could not run at all" are both answers about the
// entity, and only the first has findings. An entity with a clean bill of health and an entity
// nothing examined are different states, and a findings-only view collapses them.
type EntityView struct {
	Subject Subject
	// Items that produced a finding about this subject, each carrying ONLY that subject's findings.
	// The item's outcome is left exactly as the run decided it: an item that failed on twelve nets
	// still reads as failed here, because the outcome is a property of the item, not of the view.
	Items []ItemResult
	// Blocked lists items whose outcome could not be reached at all (needs-data and its siblings).
	// These have no findings by construction and are the reason an entity view cannot be built from
	// findings alone: the most actionable thing about an entity is often that nothing could check it.
	Blocked []ItemResult
	// Unmet is every datasheet fact the blocked items named, deduplicated across them. A caller
	// resolving gaps wants the set, not one copy per item that tripped over it.
	Unmet []check.UnmetDependency
}

// blockedOutcome reports whether an outcome means the item never reached a verdict. Kept as one
// predicate so the entity view and any future rollup agree on what "blocked" means rather than each
// enumerating the vocabulary and drifting.
func blockedOutcome(o Outcome) bool {
	switch o {
	case NeedsData, NeedsDesignIntent, NotAutomated, Inconclusive:
		return true
	}
	return false
}

// ForSubjects projects a report onto a set of entities.
//
// An entity that no item mentions yields an EntityView with nothing in it rather than being omitted,
// because "nothing examined this" is an answer and a silently absent entry is not. That is the same
// discipline the outcome vocabulary applies one layer up: silence must never read as coverage.
//
// Blocked items are attached to EVERY requested subject rather than to the entity that caused them.
// A needs-data item did not evaluate, so it has no findings and therefore no subject; claiming
// otherwise would invent an attribution the run never made. The honest reading is that the item
// could not answer for anything, including this entity.
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

// ForSubject is the one-entity case of ForSubjects. It always returns a view, never nil: an entity
// nothing examined is a legitimate and informative answer.
func ForSubject(r Report, s Subject) EntityView {
	return ForSubjects(r, []Subject{s})[0]
}

// dedupeUnmet collapses the unmet dependencies of several items into the set of facts to go and find.
// One missing fact commonly blocks several items, and a caller closing gaps wants each fact once.
// Sorted so a rendered view is stable between runs.
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

// SubjectsOf enumerates every entity the report has something to say about, so a caller can offer a
// list without walking findings itself.
//
// This is NOT the set of entities on the design, and the difference is the whole reason an entity
// view cannot substitute for a review pass: a design entity that no rule examined appears nowhere
// here. Clicking enumerates attention; a review pass enumerates the design.
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
