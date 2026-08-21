package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/panyam/agni/core/check"
	rpt "github.com/panyam/agni/core/report"
)

// textWrapWidth is where a proof sentence folds. 96 rather than 80 because a terminal running this
// is usually wider than a punched card and the statements are long, and because the alternative to
// wrapping is truncating, which loses the half of the sentence carrying the numbers.
const textWrapWidth = 96

// writeVerdictText renders the run for a terminal, grouped by rule.
//
// GROUPED RATHER THAN COLUMNED, and the arithmetic decided it (agni issue 402). The flat table had no
// rule column at all, so three rules reaching three conclusions about SCL printed three rows nothing
// distinguished. Adding a column was the obvious fix and does not fit: the longest rule name in the
// catalog is 31 characters, which with the outcome and subject columns leaves about 21 for the proof
// on an 80-column terminal, and the proofs run 40 to 90. Under a heading the rule is stated once and
// the sentence keeps the width.
//
// IT RENDERS THE SAME report.Report THE HTML FORM DOES, so the two cannot disagree about what a run
// contained or what order to meet it in. That is the drift agni issue 380 describes, and this is the
// second renderer over the shared model rather than a second aggregation beside it.
//
// The ordering comes with it and is the half not asked for in the issue. Rules with something to act
// on come first, and within a rule the failures precede the passes. On the tutorial board the old flat
// output buried 11 failures among 180 alphabetically-ordered rows.
func writeVerdictText(w io.Writer, rep rpt.Report) {
	if len(rep.Rules) == 0 {
		fmt.Fprintln(w, "No rule reported a considered set. Only some rules state one; see --verdicts.")
		return
	}
	// Outcome width is measured across the WHOLE report rather than per rule, so the columns line up
	// when a reader scans past a heading. A run with nothing undecided therefore spends four columns
	// on it rather than the fourteen "not-considered" would reserve.
	outcomeWidth := 0
	for _, r := range rep.Rules {
		for _, row := range r.Rows {
			if n := len(outcomeWord(row.Outcome)); n > outcomeWidth {
				outcomeWidth = n
			}
		}
	}
	for i, r := range rep.Rules {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "%s  %s\n", r.Name, ruleTally(r))
		if !r.StatesConsideredSet {
			// The same sentence the HTML report carries, for the same reason: these rows are what the
			// rule FOUND, not what it checked, and a reader scanning a column of outcomes will read
			// them as coverage unless told otherwise.
			fmt.Fprintln(w, "  (reports violations only; absence here is not evidence of correctness)")
		}
		width := 0
		for _, row := range r.Rows {
			if n := len(row.SubjectLabel()); n > width {
				width = n
			}
		}
		for _, row := range r.Rows {
			indent := 4 + outcomeWidth + 2 + width + 2
			detail := wrapText(row.Detail(), textWrapWidth-indent, indent)
			fmt.Fprintf(w, "    %-*s  %-*s  %s\n", outcomeWidth, outcomeWord(row.Outcome), width, row.SubjectLabel(), detail)
			// The link goes on its own line rather than in a column. It runs past sixty characters,
			// so a column of them would push the proof off the terminal, and it is absent by default:
			// a row only carries one when the operator named a viewer with --url-base AND the design
			// is one that viewer could resolve.
			if row.URL != "" {
				fmt.Fprintf(w, "    %s%s\n", strings.Repeat(" ", indent-4), row.URL)
			}
		}
	}
	fmt.Fprintf(w, "\n%d verdicts across %d rule(s)", rep.Totals.Considered, rep.Totals.RulesReporting)
	for _, o := range []struct {
		n int
		w string
	}{
		{rep.Totals.Pass, "pass"}, {rep.Totals.Fail, "fail"}, {rep.Totals.Inconclusive, "inconclusive"},
		{rep.Totals.NoLimit, "no-limit"}, {rep.Totals.NotConsidered, "not-considered"},
	} {
		if o.n > 0 {
			fmt.Fprintf(w, ", %d %s", o.n, o.w)
		}
	}
	if rep.Totals.RulesFindingsOnly > 0 {
		fmt.Fprintf(w, " (%d rule(s) reported findings only)", rep.Totals.RulesFindingsOnly)
	}
	fmt.Fprintln(w)
}

// ruleTally is the heading's right-hand side: what this rule concluded, worst first, so the number a
// reader acts on is the one they meet.
func ruleTally(r rpt.RuleReport) string {
	var parts []string
	for _, o := range []check.Outcome{check.Fail, check.Inconclusive, check.NoLimit, check.NotConsidered, check.Pass} {
		if n := r.Counts[o]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, outcomeWord(o)))
		}
	}
	return strings.Join(parts, ", ")
}

// outcomeWord is the vocabulary word for an outcome. It exists so the text form spells an outcome the
// way the csv and the wire do rather than rendering the Go constant, which would leak a spelling no
// other surface uses.
func outcomeWord(o check.Outcome) string {
	switch o {
	case check.Pass:
		return "pass"
	case check.Fail:
		return "fail"
	case check.NoLimit:
		return "no-limit"
	case check.NotConsidered:
		return "not-considered"
	case check.Inconclusive:
		return "inconclusive"
	}
	return "unspecified"
}

// wrapText folds a sentence at width, indenting every line after the first so it stays inside the
// detail column. A width at or below zero returns the sentence unfolded, which is the honest answer
// for a subject so long that there is no column left: truncating would drop the numbers.
func wrapText(s string, width, indent int) string {
	if width <= 0 || len(s) <= width {
		return s
	}
	var out strings.Builder
	line := 0
	for i, word := range strings.Fields(s) {
		switch {
		case i == 0:
			out.WriteString(word)
			line = len(word)
		case line+1+len(word) > width:
			out.WriteString("\n" + strings.Repeat(" ", indent) + word)
			line = len(word)
		default:
			out.WriteString(" " + word)
			line += 1 + len(word)
		}
	}
	return out.String()
}
