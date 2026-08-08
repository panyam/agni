package review

import (
	"fmt"
	"strings"
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

func (t Tally) String() string {
	s := fmt.Sprintf("%d pass, %d fail, %d n/a, %d not-automated", t.Pass, t.Fail, t.NotApplicable, t.NotAutomated)
	// Show the ratification states only when present, so an unseeded review reads exactly as before.
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
// (item -> outcome -> the findings that failed it). It is the per-design analogue of the coverage
// matrix — organized by the project's review areas, not by severity.
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
// its automation coverage (how many items a shipped rule covers) and, among those, the pass/fail/na
// split — plus a totals row. It is the scannable summary of the same run RenderMarkdown details item
// by item; "automated" = the item's binding resolved to a rule (pass, fail, or not-applicable), so
// not-automated is exactly the checklist still needing a rule.
func RenderCoverageMarkdown(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Review coverage: %s\n\n", r.Manifest)
	if r.Design != "" {
		fmt.Fprintf(&b, "Design: `%s`\n\n", r.Design)
	}
	tot := r.Tally()
	// Two axes (WS10-014): coverage (a mechanism exists) then, among covered items, the ratification
	// breakdown — provisional (awaiting a datasheet value) and needs-intent (awaiting a declaration) are
	// the HITL worklists, distinct from a clean pass/fail or a genuine not-automated.
	fmt.Fprintf(&b, "**%d of %d covered** — %d pass, %d fail, %d n/a; %d not-automated",
		tot.Covered(), tot.Total, tot.Pass, tot.Fail, tot.NotApplicable, tot.NotAutomated)
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
		var at Tally
		for _, it := range a.Items {
			at.add(it.Outcome)
		}
		row(a.Area.Name, at)
	}
	row("**Total**", tot)
	return b.String()
}

// maxDetailFindings caps how many findings a failing item's Detail cell lists inline. A broad rule
// (esd, unconnected-pin) can fire on hundreds of nets; dumping them all makes one unreadable markdown
// cell (a real automotive EVT esd item failed on 250+ nets, a 100KB line). The cap is a MARKDOWN-RENDERING
// choice only: ItemResult.Findings still carries the full list, which the future JSON/web report
// surfaces in full — no data is lost here.
const maxDetailFindings = 3

// detail is the last column: the findings for a failed item, the reason (plus any manifest note) for
// a not-applicable one, or the manifest Note for anything else (why an item is not automated, or a
// caveat on a passing one).
func detail(it ItemResult) string {
	switch it.Outcome {
	case Fail, Provisional:
		// Provisional carries the same firing findings as a Fail (it IS a fail, on unratified data), so
		// it renders its findings the same way; the "provisional" outcome word already flags the caveat.
		n := len(it.Findings)
		shown := n
		if shown > maxDetailFindings {
			shown = maxDetailFindings
		}
		parts := make([]string, 0, shown)
		for _, f := range it.Findings[:shown] {
			parts = append(parts, fmt.Sprintf("%s: %s (%s)", f.Rule, f.Subject, f.Message))
		}
		s := strings.Join(parts, "; ")
		if n > shown {
			s += fmt.Sprintf("; (+%d more)", n-shown)
		}
		return s
	case NotApplicable, ComputedNA, NeedsDesignIntent, NeedsData, Inconclusive, NotAutomated:
		// NotAutomated may carry a runtime reason too (WS3-090: a host-bound interface declared on no
		// component), and NeedsData carries the unseeded-symbol reason (WS3-097), so join the runtime
		// note with any manifest note rather than showing the manifest note alone.
		return JoinNonEmpty(it.Note, it.Item.Note)
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
