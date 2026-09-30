package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/panyam/agni/core/check"
	rpt "github.com/panyam/agni/core/report"
)

// textWrapWidth is where a proof sentence folds. 96 rather than 80 because terminals are usually wider
// and the statements are long. It wraps rather than truncates, since truncating loses the numbers.
const textWrapWidth = 96

// writeVerdictText renders the run for a terminal, grouped by rule.
//
// GROUPED RATHER THAN COLUMNED (agni issue 402), so three rules reaching three conclusions about one
// subject stay distinguishable. A rule column does not fit, because the longest rule name is 31
// characters, which with the outcome and subject columns leaves about 21 for the proof on an 80-column
// terminal, and the proofs run 40 to 90. Under a heading the rule is stated once and the sentence keeps
// the width.
//
// IT RENDERS THE SAME report.Report THE HTML FORM DOES, so the two cannot disagree about what a run
// contained or its order (agni issue 380).
//
// Rules with something to act on come first, and within a rule the failures precede the passes. On the
// tutorial board a flat alphabetical list buried 11 failures among 180 rows.
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
			// The same sentence the HTML report carries. These rows are what the rule FOUND, not what
			// it checked, and a reader scanning a column of outcomes reads them as coverage unless told.
			fmt.Fprintln(w, "  (reports violations only; absence here is not evidence of correctness)")
		}
		width := 0
		for _, row := range r.Rows {
			if n := len(row.SubjectLabel()); n > width {
				width = n
			}
		}
		shown, elided := 0, 0
		for _, row := range r.Rows {
			if quietOutcome(row.Outcome) {
				if shown >= verdictQuietRowLimit {
					elided++
					continue
				}
				shown++
			}
			indent := 4 + outcomeWidth + 2 + width + 2
			detail := wrapText(row.Detail(), textWrapWidth-indent, indent)
			fmt.Fprintf(w, "    %-*s  %-*s  %s\n", outcomeWidth, outcomeWord(row.Outcome), width, row.SubjectLabel(), detail)
			// The link goes on its own line because it runs past sixty characters. A row carries one
			// only when --server named a viewer AND the design is one that viewer could resolve.
			if row.URL != "" {
				fmt.Fprintf(w, "    %s%s\n", strings.Repeat(" ", indent-4), row.URL)
			}
		}
		if elided > 0 {
			// State the count and point at the heading tally, because a list that stops without
			// saying so reads as the whole answer. Same shape as TraceNet.StubsElided on trace output.
			fmt.Fprintf(w, "    ... and %d more, not shown here. The tally beside the rule name is the full count.\n", elided)
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

// verdictQuietRowLimit caps how many NON-ACTIONABLE rows one rule prints: the passes, the
// not-considered and the no-limits. A rule that examined a thousand subjects and cleared them says so
// in its heading tally, and printing all thousand pushes every other rule off the screen.
//
// It is not a cap on the whole list. A fail or an inconclusive is a row someone has to act on, so
// those are never elided however many there are.
//
// Twenty is above every committed tutorial capture (the largest is sixteen), so no capture moves, and
// far below a real board, where one sample reports 351 unconnected pins under a single rule (agni
// issue 644).
const verdictQuietRowLimit = 20

// quietOutcome reports an outcome nobody has to act on, which is what makes a row safe to elide.
func quietOutcome(o check.Outcome) bool {
	switch o {
	case check.Fail, check.Inconclusive:
		return false
	}
	return true
}

// ruleTally is the heading's right-hand side, what this rule concluded, worst first, so the number a
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

// outcomeWord is the vocabulary word for an outcome, spelled the way the csv and the wire spell it
// rather than as the Go constant.
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
// detail column. A width at or below zero, a subject so long there is no column left, returns the
// sentence unfolded rather than truncating away the numbers.
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
