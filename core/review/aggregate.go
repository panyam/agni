package review

import (
	"fmt"
	"strings"
)

// Aggregate is a multi-design review rollup: the same manifest run against N designs, one Report each.
// Automation is MANIFEST-level, since the catalog is design-independent, so summing per-design tallies
// would multiply the automated/not-automated counts by N. Pass/fail/n-a is PER DESIGN. A renderer
// states automation once and the outcome per design.
type Aggregate struct {
	Manifest string
	Reports  []Report // one per design, in the order the designs were given (= column order)
}

// designs returns each report's design label, in column order.
func (a Aggregate) designs() []string {
	out := make([]string, len(a.Reports))
	for i, r := range a.Reports {
		out[i] = r.Design
	}
	return out
}

// outcomeByID indexes each design's item outcomes by item ID (all reports share the manifest's item
// set, so this is how the matrix looks up one item's outcome per design).
func (a Aggregate) outcomeByID() []map[string]Outcome {
	ms := make([]map[string]Outcome, len(a.Reports))
	for i, r := range a.Reports {
		m := map[string]Outcome{}
		for _, ar := range r.Areas {
			for _, it := range ar.Items {
				m[it.Item.ID] = it.Outcome
			}
		}
		ms[i] = m
	}
	return ms
}

// writeAggregateSummary writes the manifest-level automation header and the per-design Pass/Fail/N-A
// table (with a project total of just those per-design cells). Shared by the coverage and full markdown
// renderers.
func writeAggregateSummary(b *strings.Builder, a Aggregate) {
	fmt.Fprintf(b, "# Review rollup: %s\n\n", a.Manifest)
	fmt.Fprintf(b, "%d designs.\n\n", len(a.Reports))
	if len(a.Reports) > 0 {
		t0 := a.Reports[0].Tally() // coverage is manifest-level: identical across designs
		fmt.Fprintf(b, "**%d of %d items covered** (manifest-level), %d not-automated. Pass/fail/n-a and the data-trust states (provisional/needs-intent/computed-n/a) are per design.\n\n",
			t0.Covered(), t0.Total, t0.NotAutomated)
	}
	b.WriteString("## Per-design outcomes\n\n")
	// Answered is per DESIGN where coverage is per manifest, because a rule's inputs are a property of
	// the board rather than of the checklist. A rollup gate reads this column.
	b.WriteString("| Design | Answered | Pass | Fail | Provisional | Needs-intent | Needs-data | Computed-n/a | N/A |\n")
	b.WriteString("|--------|----------|------|------|-------------|--------------|------------|--------------|-----|\n")
	var tot Tally
	for _, r := range a.Reports {
		t := r.Tally()
		fmt.Fprintf(b, "| `%s` | %d/%d | %d | %d | %d | %d | %d | %d | %d |\n",
			r.Design, t.Answered(), t.Total, t.Pass, t.Fail, t.Provisional, t.NeedsDesignIntent, t.NeedsData, t.ComputedNA, t.NotApplicable)
		tot.Pass, tot.Fail, tot.NotApplicable = tot.Pass+t.Pass, tot.Fail+t.Fail, tot.NotApplicable+t.NotApplicable
		tot.Provisional, tot.NeedsDesignIntent, tot.ComputedNA = tot.Provisional+t.Provisional, tot.NeedsDesignIntent+t.NeedsDesignIntent, tot.ComputedNA+t.ComputedNA
		tot.NeedsData += t.NeedsData
		// Total accumulates only so the answered cell can read x/y like the per-design rows.
		tot.Total += t.Total
	}
	fmt.Fprintf(b, "| **Total** | %d/%d | %d | %d | %d | %d | %d | %d | %d |\n\n",
		tot.Answered(), tot.Total, tot.Pass, tot.Fail, tot.Provisional, tot.NeedsDesignIntent, tot.NeedsData, tot.ComputedNA, tot.NotApplicable)
}

// RenderAggregateCoverageMarkdown is the multi-design analogue of RenderCoverageMarkdown: the automation
// header plus the per-design outcome summary, without the per-item matrix. It backs `review <designs...>
// --coverage`.
func RenderAggregateCoverageMarkdown(a Aggregate) string {
	var b strings.Builder
	writeAggregateSummary(&b, a)
	return b.String()
}

// RenderAggregateMarkdown is the full project rollup: the summary above, then a per-item traceability
// matrix (rows = checklist items grouped by review area, columns = designs, cells = the item's outcome
// on that design), so each ask reads across every reference design.
func RenderAggregateMarkdown(a Aggregate) string {
	var b strings.Builder
	writeAggregateSummary(&b, a)
	b.WriteString("## Traceability matrix\n\n")
	if len(a.Reports) == 0 {
		return b.String()
	}
	ms := a.outcomeByID()
	designs := a.designs()
	for _, ar := range a.Reports[0].Areas { // all reports share the manifest structure
		fmt.Fprintf(&b, "### %s\n\n", ar.Area.Name)
		b.WriteString("| # | Item |")
		for _, d := range designs {
			fmt.Fprintf(&b, " %s |", d)
		}
		b.WriteString("\n|---|------|")
		for range designs {
			b.WriteString("-----|")
		}
		b.WriteString("\n")
		for _, it := range ar.Items {
			fmt.Fprintf(&b, "| %s | %s |", it.Item.ID, it.Item.Title)
			for _, m := range ms {
				fmt.Fprintf(&b, " %s |", m[it.Item.ID])
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}
