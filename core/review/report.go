package review

import (
	"fmt"
	"strings"

	"github.com/panyam/agni/core/check"
)

// Tally counts item outcomes across a report (or one area). It carries both axes (WS10-014): the
// pass/fail/n-a/not-automated coverage counts and the ratification counts (Provisional, NeedsDesignIntent,
// ComputedNA). Covered() derives "a mechanism exists" (everything but NotAutomated).
type Tally struct {
	Pass, Fail, NotApplicable, NotAutomated, Total int
	Provisional, NeedsDesignIntent, ComputedNA     int
	NeedsData                                      int
	Inconclusive                                   int
}

func (t *Tally) add(o Outcome) {
	t.Total++
	switch o {
	case Pass:
		t.Pass++
	case Fail:
		t.Fail++
	case NotApplicable:
		t.NotApplicable++
	case NotAutomated:
		t.NotAutomated++
	case Provisional:
		t.Provisional++
	case NeedsDesignIntent:
		t.NeedsDesignIntent++
	case ComputedNA:
		t.ComputedNA++
	case NeedsData:
		t.NeedsData++
	case Inconclusive:
		t.Inconclusive++
	}
}

// Covered is the coverage axis: how many items a mechanism exists for (every outcome but not-automated).
func (t Tally) Covered() int { return t.Total - t.NotAutomated }

// Answered is the stricter axis: how many items the run actually produced an answer for.
//
// Covered() subtracts only NotAutomated, which moves when a rule leaves the CATALOG. It does not move
// when a rule is present but its INPUTS are gone, as when a datasheet-backed item whose corpus
// vanished fails check.Available and reads not-applicable, which Covered() counts. Removing `params/` from the
// tutorial project moves Covered() by zero while an item stops being answered.
//
// Pass, Fail and Provisional are verdicts. ComputedNA is also an answer, because the DESIGN determined
// the item does not apply (no crystal on the board). Everything else is unanswered: NotApplicable,
// NotAutomated, NeedsData, NeedsDesignIntent, and Inconclusive. See
// docsite/content/architecture/checks-contract.md#covered-and-answered-are-two-numbers.
func (t Tally) Answered() int { return t.Pass + t.Fail + t.Provisional + t.ComputedNA }

func (t Tally) String() string {
	s := fmt.Sprintf("%d pass, %d fail, %d n/a, %d not-automated", t.Pass, t.Fail, t.NotApplicable, t.NotAutomated)
	// Show the ratification states only when present, so an unseeded review reads as a plain tally.
	if t.Provisional > 0 {
		s += fmt.Sprintf(", %d provisional", t.Provisional)
	}
	if t.NeedsDesignIntent > 0 {
		s += fmt.Sprintf(", %d needs-intent", t.NeedsDesignIntent)
	}
	if t.Inconclusive > 0 {
		s += fmt.Sprintf(", %d inconclusive", t.Inconclusive)
	}
	if t.NeedsData > 0 {
		s += fmt.Sprintf(", %d needs-data", t.NeedsData)
	}
	if t.ComputedNA > 0 {
		s += fmt.Sprintf(", %d computed-n/a", t.ComputedNA)
	}
	return s + fmt.Sprintf(" (of %d)", t.Total)
}

// Tally sums one area's outcomes, for the markdown rollup and the HTML checklist.
func (a AreaResult) Tally() Tally {
	var t Tally
	for _, it := range a.Items {
		t.add(it.Outcome)
	}
	return t
}

// Tally sums the outcomes across every area of the report.
func (r Report) Tally() Tally {
	var t Tally
	for _, a := range r.Areas {
		for _, it := range a.Items {
			t.add(it.Outcome)
		}
	}
	return t
}

// RenderMarkdown renders a per-design review report: an overall tally, then one table per review area
// (item -> outcome -> the findings that failed it). It is organized by the project's review areas,
// not by severity.
func RenderMarkdown(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Review: %s\n\n", r.Manifest)
	if r.Design != "" {
		fmt.Fprintf(&b, "Design: `%s`\n\n", r.Design)
	}
	fmt.Fprintf(&b, "**%s**\n", r.Tally())
	for _, a := range r.Areas {
		var at Tally
		for _, it := range a.Items {
			at.add(it.Outcome)
		}
		fmt.Fprintf(&b, "\n## %s\n\n%s\n\n", a.Area.Name, at)
		b.WriteString("| # | Title | Outcome | Detail |\n|---|-------|---------|--------|\n")
		for _, it := range a.Items {
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n",
				it.Item.ID, it.Item.Title, it.Outcome, detail(it))
		}
	}
	return b.String()
}

// RenderCoverageMarkdown renders a compact per-design coverage rollup: one row per review area with
// its automation coverage and the outcome split, plus a totals row. It summarizes the same run
// RenderMarkdown details item by item. "Automated" means the item's binding resolved to a rule, so
// not-automated is exactly the checklist still needing a rule.
func RenderCoverageMarkdown(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Review coverage: %s\n\n", r.Manifest)
	if r.Design != "" {
		fmt.Fprintf(&b, "Design: `%s`\n\n", r.Design)
	}
	tot := r.Tally()
	// Two axes (WS10-014): coverage, then the ratification breakdown among covered items. Provisional
	// (awaiting a datasheet value) and needs-intent (awaiting a declaration) are the HITL worklists.
	fmt.Fprintf(&b, "**%d of %d covered**, **%d answered** — %d pass, %d fail, %d n/a; %d not-automated",
		tot.Covered(), tot.Total, tot.Answered(), tot.Pass, tot.Fail, tot.NotApplicable, tot.NotAutomated)
	if tot.Provisional > 0 || tot.NeedsDesignIntent > 0 || tot.NeedsData > 0 || tot.ComputedNA > 0 || tot.Inconclusive > 0 {
		fmt.Fprintf(&b, "\n\nOf the covered: %d provisional (awaiting datasheet data), %d needs-design-intent (awaiting a declaration), %d needs-data (awaiting a datasheet seed), %d inconclusive (the check ran and could not decide), %d computed-n/a",
			tot.Provisional, tot.NeedsDesignIntent, tot.NeedsData, tot.Inconclusive, tot.ComputedNA)
	}
	b.WriteString("\n\n")
	b.WriteString("| Area | Covered | Pass | Fail | Provisional | Needs-intent | Needs-data | Inconclusive | Computed-n/a | N/A | Not-automated |\n")
	b.WriteString("|------|---------|------|------|-------------|--------------|------------|--------------|--------------|-----|---------------|\n")
	row := func(name string, t Tally) {
		fmt.Fprintf(&b, "| %s | %d/%d | %d | %d | %d | %d | %d | %d | %d | %d | %d |\n",
			name, t.Covered(), t.Total, t.Pass, t.Fail, t.Provisional, t.NeedsDesignIntent, t.NeedsData, t.Inconclusive, t.ComputedNA, t.NotApplicable, t.NotAutomated)
	}
	for _, a := range r.Areas {
		row(a.Area.Name, a.Tally())
	}
	row("**Total**", tot)
	return b.String()
}

// maxDetailFindings caps how many findings a failing item's Detail cell lists inline. A broad rule
// can fire on hundreds of nets (a real esd item failed on 250+ nets, a 100KB line). The cap is
// markdown-only, and ItemResult.Findings keeps the full list, which RenderJSON emits.
const maxDetailFindings = 3

// detail is the last column: the findings for a failed item, the reason (plus any manifest note) for
// a not-applicable one, or the manifest Note for anything else (why an item is not automated, or a
// caveat on a passing one).
func detail(it ItemResult) string {
	switch it.Outcome {
	case Fail, Provisional:
		// Provisional IS a fail on unratified data, so it renders its findings the same way and the
		// outcome word flags the caveat.
		n := len(it.Findings)
		shown := n
		if shown > maxDetailFindings {
			shown = maxDetailFindings
		}
		parts := make([]string, 0, shown)
		for _, f := range it.Findings[:shown] {
			parts = append(parts, fmt.Sprintf("%s: %s (%s)", f.Rule, check.EntityRef(f.Subject), f.Message))
		}
		s := strings.Join(parts, "; ")
		if n > shown {
			s += fmt.Sprintf("; (+%d more)", n-shown)
		}
		return s
	case NotApplicable, ComputedNA, NeedsDesignIntent, NeedsData, Inconclusive, NotAutomated:
		// NotAutomated may carry a runtime reason (WS3-090: a host-bound interface declared on no
		// component), and NeedsData the unseeded-symbol reason (WS3-097), so join the runtime note with
		// any manifest note. For needs-data, the note names WHICH SYMBOL is missing design-wide and the
		// unmet list names WHICH PARTS need it.
		return JoinNonEmpty(it.Note, unmetSummary(it.Unmet), it.Item.Note)
	default: // Pass
		return it.Item.Note
	}
}

// JoinNonEmpty joins the non-empty parts with "; ".
func JoinNonEmpty(parts ...string) string {
	kept := parts[:0]
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, "; ")
}

// maxUnmetListed caps the parts a needs-data cell names inline, like maxDetailFindings, since a
// common symbol can be unseeded on dozens of parts. ItemResult.Unmet keeps the full list.
const maxUnmetListed = 5

// unmetSummary renders the parts a needs-data item needs seeded, as "seed X on A, B". The
// spec-absent case is called out separately because a part with no spec at all needs a document
// extracted before any symbol can be found in it.
func unmetSummary(deps []check.UnmetDependency) string {
	if len(deps) == 0 {
		return ""
	}
	shown := len(deps)
	if shown > maxUnmetListed {
		shown = maxUnmetListed
	}
	parts := make([]string, 0, shown)
	for _, d := range deps[:shown] {
		s := fmt.Sprintf("%s on %s", d.Symbol, d.MPN)
		if d.SpecAbsent {
			s += " (no spec)"
		}
		parts = append(parts, s)
	}
	s := "seed " + strings.Join(parts, ", ")
	if len(deps) > shown {
		s += fmt.Sprintf(" (+%d more)", len(deps)-shown)
	}
	return s
}
