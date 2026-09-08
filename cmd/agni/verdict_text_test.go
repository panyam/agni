package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	rpt "github.com/panyam/agni/core/report"
)

// ruleOf returns the rule heading each line of the grouped text output sits under, which is the whole
// question agni issue 402 asks: a row that cannot be attributed answers half of "what did you check".
func ruleOf(out string) map[string]string {
	rules := map[string]string{}
	current := ""
	for _, line := range strings.Split(out, "\n") {
		switch {
		case line == "" || strings.HasPrefix(line, "  ("):
			continue
		case !strings.HasPrefix(line, " "):
			current = strings.Fields(line)[0]
		default:
			rules[strings.TrimSpace(line)] = current
		}
	}
	return rules
}

// The issue's exact case. Several rules reach several conclusions about one net, and the flat table
// printed them as consecutive rows with nothing saying who spoke.
func TestVerdictTextAttributesEveryRowToItsRule(t *testing.T) {
	out := runCheck(t, "--verdicts", "testdata/conformance/showcase.fires.kicad_sch")

	rows := ruleOf(out)
	var onSCL []string
	for row, rule := range rows {
		if strings.Contains(row, " SCL ") || strings.HasSuffix(row, " SCL") {
			onSCL = append(onSCL, rule)
		}
	}
	if len(onSCL) < 2 {
		t.Fatalf("want several rules answering about SCL, got %v\n%s", onSCL, out)
	}
	seen := map[string]bool{}
	for _, r := range onSCL {
		if r == "" {
			t.Errorf("a row about SCL sits under no rule heading\n%s", out)
		}
		seen[r] = true
	}
	if len(seen) < 2 {
		t.Errorf("every row about SCL is attributed to one rule (%v); the point is that several looked", seen)
	}
}

// A reader opening a run with four failures among 125 rows should meet the four. The flat form
// ordered rules alphabetically, so a failure landed wherever its rule's name happened to sort.
func TestVerdictTextLeadsWithWhatFailed(t *testing.T) {
	out := runCheck(t, "--verdicts", "testdata/conformance/showcase.fires.kicad_sch")

	firstFail, firstPass := strings.Index(out, "\n    fail"), strings.Index(out, "\n    pass")
	if firstFail < 0 || firstPass < 0 {
		t.Fatalf("want both a failing and a passing row\n%s", out)
	}
	if firstFail > firstPass {
		t.Errorf("a passing row precedes every failing one; failures must lead\n%s", out)
	}
	// And within a rule: esd-protection has two fails and a pass, in that order.
	sec := section(t, out, "esd-protection")
	if f, p := strings.Index(sec, "fail"), strings.Index(sec, "pass"); f > p {
		t.Errorf("esd-protection lists its pass before its failures:\n%s", sec)
	}
}

// A findings-only rule's rows are what it FOUND, not what it checked. Without the note a reader
// scanning a column of outcomes reads them as coverage, which is the claim the verdict work removes.
func TestVerdictTextMarksAFindingsOnlyRule(t *testing.T) {
	out := runCheck(t, "--verdicts", "testdata/conformance/dangling.fires.kicad_sch")
	sec := section(t, out, "dangling-endpoint")
	if !strings.Contains(sec, "violations only") {
		t.Errorf("dangling-endpoint states no considered set and must say so:\n%s", sec)
	}
}

// A relation-shaped rule names every entity in its subject, and the terminal must show all of them.
// This is the regression guard the text format did not have when the subject became a tuple: nothing
// asserted on this output at all, so the change went in unobserved.
func TestVerdictTextShowsTheWholeSubjectTuple(t *testing.T) {
	out := runCheck(t, "--verdicts", "--params", "testdata/conformance/fetparams",
		"testdata/conformance/fetvdss.fires.edn")
	sec := section(t, out, "fet-vdss-below-switched-rail")
	if !strings.Contains(sec, "Q1 + +60V") {
		t.Errorf("the row must name both the part and the rail, not one of them:\n%s", sec)
	}
}

// The terminal and the page render one report, so they cannot disagree about what the run contained
// or what order to meet it in. That drift is what agni issue 380 describes and what core/report
// exists to prevent; this pins that the second renderer actually uses it.
func TestVerdictTextAndHTMLAgreeOnRuleOrder(t *testing.T) {
	const design = "testdata/conformance/showcase.fires.kicad_sch"
	text := runCheck(t, "--verdicts", design)
	html := runCheck(t, "--verdicts", "--format", "html", design)

	var textOrder []string
	for _, line := range strings.Split(text, "\n") {
		if line != "" && !strings.HasPrefix(line, " ") && strings.Contains(line, "  ") {
			textOrder = append(textOrder, strings.Fields(line)[0])
		}
	}
	if len(textOrder) < 3 {
		t.Fatalf("want several rule headings, got %v", textOrder)
	}
	at := -1
	for _, name := range textOrder {
		i := strings.Index(html, ">"+name+"<")
		if i < 0 {
			t.Errorf("rule %q is in the terminal output and not in the page", name)
			continue
		}
		if i < at {
			t.Errorf("rule %q appears in a different position in the two renderers", name)
		}
		at = i
	}
}

// section returns one rule's block of the grouped output, for a test making claims about that rule.
func section(t *testing.T, out, rule string) string {
	t.Helper()
	rest := out
	if !strings.HasPrefix(out, rule+"  ") {
		i := strings.Index(out, "\n"+rule+"  ")
		if i < 0 {
			t.Fatalf("no section for %q in:\n%s", rule, out)
		}
		rest = out[i+1:]
	}
	if j := strings.Index(rest, "\n\n"); j >= 0 {
		return rest[:j]
	}
	return rest
}

// bigRule builds one rule's section with the given number of each outcome, for exercising the row
// cap without needing a design that produces a thousand subjects.
func bigRule(passes, notConsidered, fails int) rpt.RuleReport {
	r := rpt.RuleReport{
		Name: "big-rule", StatesConsideredSet: true,
		Counts: map[check.Outcome]int{
			check.Pass: passes, check.NotConsidered: notConsidered, check.Fail: fails,
		},
	}
	// Worst first, which is the order the report builder produces and the order the cap relies on.
	for i := 0; i < fails; i++ {
		r.Rows = append(r.Rows, rpt.Row{Outcome: check.Fail,
			Subjects: []check.Entity{check.NetNameEntity(fmt.Sprintf("BAD%d", i))}, Message: "broken"})
	}
	for i := 0; i < notConsidered; i++ {
		r.Rows = append(r.Rows, rpt.Row{Outcome: check.NotConsidered,
			Subjects: []check.Entity{check.NetNameEntity(fmt.Sprintf("UNK%d", i))}, Reason: "not asked"})
	}
	for i := 0; i < passes; i++ {
		r.Rows = append(r.Rows, rpt.Row{Outcome: check.Pass,
			Subjects: []check.Entity{check.NetNameEntity(fmt.Sprintf("OK%d", i))}, Witness: "fine"})
	}
	return r
}

func renderRule(r rpt.RuleReport) string {
	var b strings.Builder
	writeVerdictText(&b, rpt.Report{Rules: []rpt.RuleReport{r}})
	return b.String()
}

// A rule that examined a thousand subjects and cleared them says so in its heading, and printing all
// thousand pushes every other rule off the screen. One sample board reports 351 unconnected pins
// under a single rule, which is the shape agni issue 644 describes.
func TestVerdictTextCapsNonActionableRows(t *testing.T) {
	out := renderRule(bigRule(500, 500, 0))
	shown := strings.Count(out, "OK") + strings.Count(out, "UNK")
	if shown > verdictQuietRowLimit {
		t.Errorf("printed %d non-actionable rows, want at most %d", shown, verdictQuietRowLimit)
	}
	if !strings.Contains(out, "more, not shown") {
		t.Errorf("a capped list must say it was capped, or it reads as the whole answer:\n%s", out)
	}
	// The heading is the complete count and must not be capped with the list, or the elision would
	// hide the denominator as well as the rows.
	if !strings.Contains(out, "500 not-considered") || !strings.Contains(out, "500 pass") {
		t.Errorf("the heading should carry the full tally:\n%s", firstLines(out, 3))
	}
}

// The cap must never hide a row someone has to act on. A long list is a worse read than a short one;
// a hidden failure is a wrong answer.
func TestVerdictTextNeverCapsFailures(t *testing.T) {
	out := renderRule(bigRule(0, 0, 200))
	if n := strings.Count(out, "BAD"); n != 200 {
		t.Errorf("printed %d of 200 failures; a failure must never be elided", n)
	}
	if strings.Contains(out, "more, not shown") {
		t.Errorf("nothing should have been elided:\n%s", firstLines(out, 3))
	}
}

// The mixed case is the real one: failures survive in full and the quiet rows around them are folded.
func TestVerdictTextKeepsFailuresWhileFoldingTheRest(t *testing.T) {
	out := renderRule(bigRule(400, 400, 5))
	if n := strings.Count(out, "BAD"); n != 5 {
		t.Errorf("printed %d of 5 failures, want all of them", n)
	}
	if quiet := strings.Count(out, "OK") + strings.Count(out, "UNK"); quiet > verdictQuietRowLimit {
		t.Errorf("printed %d non-actionable rows, want at most %d", quiet, verdictQuietRowLimit)
	}
	if !strings.Contains(out, "and 780 more, not shown") {
		t.Errorf("the elided count should be exact:\n%s", out)
	}
}

// A rule small enough to print whole must say nothing about eliding, or every ordinary run grows a
// line that means nothing.
func TestVerdictTextSaysNothingWhenNothingIsElided(t *testing.T) {
	out := renderRule(bigRule(3, 2, 1))
	if strings.Contains(out, "not shown") {
		t.Errorf("nothing was elided, so nothing should say so:\n%s", out)
	}
}

func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
