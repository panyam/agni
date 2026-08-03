package review

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Aggregate is a multi-design review rollup: the same manifest run against N designs, one Report each.
// It is the project-level view. Automation is MANIFEST-level — which asks a rule covers is the same for
// every design (the catalog is design-independent), so a naive sum of per-design tallies would multiply
// the automated/not-automated counts by N. Pass/fail/n-a is PER DESIGN (a rule passes on one board, is
// not-applicable where the interface is absent on another). So a renderer states automation once and
// the outcome per design.
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
	b.WriteString("| Design | Pass | Fail | Provisional | Needs-intent | Computed-n/a | N/A |\n")
	b.WriteString("|--------|------|------|-------------|--------------|--------------|-----|\n")
	var tot Tally
	for _, r := range a.Reports {
		t := r.Tally()
		fmt.Fprintf(b, "| `%s` | %d | %d | %d | %d | %d | %d |\n",
			r.Design, t.Pass, t.Fail, t.Provisional, t.NeedsDesignIntent, t.ComputedNA, t.NotApplicable)
		tot.Pass, tot.Fail, tot.NotApplicable = tot.Pass+t.Pass, tot.Fail+t.Fail, tot.NotApplicable+t.NotApplicable
		tot.Provisional, tot.NeedsDesignIntent, tot.ComputedNA = tot.Provisional+t.Provisional, tot.NeedsDesignIntent+t.NeedsDesignIntent, tot.ComputedNA+t.ComputedNA
	}
	fmt.Fprintf(b, "| **Total** | %d | %d | %d | %d | %d | %d |\n\n",
		tot.Pass, tot.Fail, tot.Provisional, tot.NeedsDesignIntent, tot.ComputedNA, tot.NotApplicable)
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
// on that design). This is the ACME-style coverage matrix — one ask per row, its outcome across every
// reference design — as a first-class engine output.
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

// jsonAggregate is the tooling-facing projection of an Aggregate, parallel to jsonReport. Automation is
// stated once (manifest-level); each item carries its per-design outcome map, and each design its
// pass/fail/n-a summary.
type jsonAggregate struct {
	Manifest     string              `json:"manifest"`
	Designs      []string            `json:"designs"`
	Total        int                 `json:"total"`
	Automated    int                 `json:"automated"`
	NotAutomated int                 `json:"not_automated"`
	PerDesign    []jsonDesignSummary `json:"per_design"`
	Areas        []jsonAggArea       `json:"areas"`
}

type jsonDesignSummary struct {
	Design            string `json:"design"`
	Pass              int    `json:"pass"`
	Fail              int    `json:"fail"`
	Provisional       int    `json:"provisional"`
	NeedsDesignIntent int    `json:"needs_design_intent"`
	ComputedNA        int    `json:"computed_na"`
	NotApplicable     int    `json:"not_applicable"`
}

type jsonAggArea struct {
	Name  string        `json:"name"`
	Items []jsonAggItem `json:"items"`
}

type jsonAggItem struct {
	ID       string            `json:"id"`
	Title    string            `json:"title"`
	Outcomes map[string]string `json:"outcomes"` // design -> outcome
}

// RenderAggregateJSON emits the project rollup as indented JSON for tooling: the manifest-level
// automation counts, a per-design summary, and the per-item outcome-by-design matrix.
func RenderAggregateJSON(a Aggregate) (string, error) {
	out := jsonAggregate{Manifest: a.Manifest, Designs: a.designs()}
	if len(a.Reports) > 0 {
		t0 := a.Reports[0].Tally()
		out.Total, out.Automated, out.NotAutomated = t0.Total, t0.Covered(), t0.NotAutomated
	}
	for _, r := range a.Reports {
		t := r.Tally()
		out.PerDesign = append(out.PerDesign, jsonDesignSummary{
			Design: r.Design, Pass: t.Pass, Fail: t.Fail, NotApplicable: t.NotApplicable,
			Provisional: t.Provisional, NeedsDesignIntent: t.NeedsDesignIntent, ComputedNA: t.ComputedNA,
		})
	}
	ms := a.outcomeByID()
	designs := a.designs()
	if len(a.Reports) > 0 {
		for _, ar := range a.Reports[0].Areas {
			ja := jsonAggArea{Name: ar.Area.Name}
			for _, it := range ar.Items {
				outcomes := map[string]string{}
				for i, d := range designs {
					outcomes[d] = string(ms[i][it.Item.ID])
				}
				ja.Items = append(ja.Items, jsonAggItem{ID: it.Item.ID, Title: it.Item.Title, Outcomes: outcomes})
			}
			out.Areas = append(out.Areas, ja)
		}
	}
	buf, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(buf) + "\n", nil
}
