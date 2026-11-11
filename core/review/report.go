package review

import (
	"fmt"
	"strings"

	"github.com/panyam/agni/core/check"
)

// Tally counts item outcomes across a report or one area. It holds both axes (WS10-014). One axis is
// the pass/fail/n-a/not-automated coverage counts. The other is the ratification counts (Provisional,
// NeedsDesignIntent, ComputedNA). Covered() derives "a mechanism exists", which is every outcome but
// NotAutomated.
type Tally struct {
	Pass, Fail, NotApplicable, NotAutomated, Total int
	Provisional, NeedsDesignIntent, ComputedNA     int
	NeedsData                                      int
	Inconclusive                                   int
	CoveredExternally                              int
}

// TallyOf counts a list of outcomes, for a caller holding outcomes rather than a Report, such as a
// stored review document.
func TallyOf(outcomes ...Outcome) Tally {
	var t Tally
	for _, o := range outcomes {
		t.add(o)
	}
	return t
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
	case CoveredExternally:
		t.CoveredExternally++
	}
}

// Covered is the coverage axis. It counts the items with a mechanism, which is every outcome but
// not-automated.
func (t Tally) Covered() int { return t.Total - t.NotAutomated }

// Answered is the stricter axis. It counts the items the run actually produced an answer for.
//
// Covered() subtracts only NotAutomated, and NotAutomated moves when a rule leaves the CATALOG. It
// does not move when a rule is present but its INPUTS are gone. Imagine the corpus behind a
// datasheet-backed item vanishes. The item fails check.Available and reads not-applicable, and
// Covered() still counts it. Removing `params/` from the tutorial project moves Covered() by zero,
// while an item stops being answered.
//
// Pass, Fail and Provisional are verdicts. ComputedNA is also an answer, because the DESIGN determined
// the item does not apply (no crystal on the board). NotApplicable, NotAutomated, NeedsData,
// NeedsDesignIntent and Inconclusive are unanswered. CoveredExternally is unanswered too, because
// another tool answered it and agni checked nothing. See
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
	if t.CoveredExternally > 0 {
		s += fmt.Sprintf(", %d covered-externally", t.CoveredExternally)
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

// RenderMarkdown renders a per-design review report. It writes an overall tally, then one table per
// review area. Each row gives an item, its outcome, and the findings that failed it. The report
// follows the project's review areas, not severity.
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

// RenderCoverageMarkdown renders a compact per-design coverage rollup. It writes one row per review
// area, with the area's automation coverage and outcome split, and then a totals row. RenderMarkdown
// details the same run item by item. "Automated" means the item's binding resolved to a rule, so
// not-automated is exactly the part of the checklist that still needs a rule.
func RenderCoverageMarkdown(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Review coverage: %s\n\n", r.Manifest)
	if r.Design != "" {
		fmt.Fprintf(&b, "Design: `%s`\n\n", r.Design)
	}
	tot := r.Tally()
	// This writes two axes (WS10-014), coverage first and then the ratification breakdown among
	// covered items. A provisional item awaits a datasheet value, and a needs-intent item awaits a
	// declaration. Those two states are the HITL worklists.
	fmt.Fprintf(&b, "**%d of %d covered**, **%d answered** — %d pass, %d fail, %d n/a; %d not-automated",
		tot.Covered(), tot.Total, tot.Answered(), tot.Pass, tot.Fail, tot.NotApplicable, tot.NotAutomated)
	if tot.Provisional > 0 || tot.NeedsDesignIntent > 0 || tot.NeedsData > 0 || tot.ComputedNA > 0 || tot.Inconclusive > 0 || tot.CoveredExternally > 0 {
		fmt.Fprintf(&b, "\n\nOf the covered: %d provisional (awaiting datasheet data), %d needs-design-intent (awaiting a declaration), %d needs-data (awaiting a datasheet seed), %d inconclusive (the check ran and could not decide), %d computed-n/a",
			tot.Provisional, tot.NeedsDesignIntent, tot.NeedsData, tot.Inconclusive, tot.ComputedNA)
		// The clause appears only on a run that read another tool's report, so a run with none reads
		// as it did before agni issue 152.
		if tot.CoveredExternally > 0 {
			fmt.Fprintf(&b, ", %d covered-externally (another tool's report shows no violation)", tot.CoveredExternally)
		}
	}
	b.WriteString("\n\n")
	ext := tot.CoveredExternally > 0
	head, rule := "| Area | Covered | Pass | Fail | Provisional | Needs-intent | Needs-data | Inconclusive | Computed-n/a | N/A | Not-automated |", "|------|---------|------|------|-------------|--------------|------------|--------------|--------------|-----|---------------|"
	if ext {
		head, rule = head+" Covered-externally |", rule+"--------------------|"
	}
	b.WriteString(head + "\n" + rule + "\n")
	row := func(name string, t Tally) {
		fmt.Fprintf(&b, "| %s | %d/%d | %d | %d | %d | %d | %d | %d | %d | %d | %d |",
			name, t.Covered(), t.Total, t.Pass, t.Fail, t.Provisional, t.NeedsDesignIntent, t.NeedsData, t.Inconclusive, t.ComputedNA, t.NotApplicable, t.NotAutomated)
		if ext {
			fmt.Fprintf(&b, " %d |", t.CoveredExternally)
		}
		b.WriteString("\n")
	}
	for _, a := range r.Areas {
		row(a.Area.Name, a.Tally())
	}
	row("**Total**", tot)
	return b.String()
}

// maxDetailFindings caps how many findings a failing item's Detail cell lists inline. A broad rule
// can fire on hundreds of nets. A real esd item failed on 250+ nets, a 100KB line. The cap applies
// to markdown only. ItemResult.Findings keeps the full list, and the Review a run returns holds all of
// it.
const maxDetailFindings = 3

// detail renders the last column. A failed item shows its findings. A not-applicable item, like the
// other unanswered outcomes, shows its reason and any manifest note. A passing item shows the manifest
// Note, such as a caveat.
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
	case NotApplicable, ComputedNA, NeedsDesignIntent, NeedsData, Inconclusive, NotAutomated, CoveredExternally:
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
// spec-absent case gets its own wording, because a part with no spec at all needs a document
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
