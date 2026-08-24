package main

import (
	"regexp"
	"strings"
	"testing"
)

const tutorialDesign = "../../examples/tutorial-project/designs/gateway/gateway.edn"

// sectionSummaries pairs every rule section in the report with the summary rendered beside it.
var sectionSummaries = regexp.MustCompile(`<span class="rule">([^<]+)</span>\s*<span class="rule-summary">([^<]*)</span>`)

// An operator's own rules must carry their prose (agni issue 411).
//
// The report looked its rules up in the catalog it was handed, and the CLI handed it the one built
// BEFORE the project's rules and the --conventions value were composed on. So a rule a team wrote for
// its own boards rendered with its rows and no summary, no impact and no remedy, under a bare name.
//
// That is backwards for a tool whose whole adoption path is teaching it your house conventions, and it
// loses the report's most valuable content on the rules a newcomer is least able to read from the name.
func TestReportCarriesProseForOperatorRules(t *testing.T) {
	out := runCheck(t, "--verdicts", "--format", "html", tutorialDesign)

	pairs := sectionSummaries.FindAllStringSubmatch(out, -1)
	if len(pairs) < 10 {
		t.Fatalf("want the tutorial project's rule sections, found %d", len(pairs))
	}
	var operator, bare []string
	for _, p := range pairs {
		name, summary := p[1], strings.TrimSpace(p[2])
		if strings.Contains(name, "/") {
			operator = append(operator, name) // namespaced: it came from an overlay source
		}
		if summary == "" {
			bare = append(bare, name)
		}
	}
	// The positive control. A run whose rules are all built-in would pass the assertion below while
	// proving nothing about the case this test exists for.
	if len(operator) == 0 {
		t.Fatal("no namespaced rule ran, so this asserted nothing about an operator's rules")
	}
	if len(bare) > 0 {
		t.Errorf("%d of %d sections render with no summary, and a section with no prose is one a "+
			"reader cannot act on: %v", len(bare), len(pairs), bare)
	}
}

// The summary is the cheapest field to get right and the least useful. Impact and remedy are what turn
// a finding into something a reviewer can act on without asking whoever wrote the rule.
func TestReportCarriesImpactAndRemedyForAnOperatorRule(t *testing.T) {
	out := runCheck(t, "--verdicts", "--format", "html", tutorialDesign)

	i := strings.Index(out, "gateway/signal-net-naming")
	if i < 0 {
		t.Fatal("the tutorial project's naming rule did not run")
	}
	section := out[i:]
	if j := strings.Index(section, `<span class="rule">`); j > 0 {
		section = section[:j]
	}
	for _, class := range []string{`class="impact"`, `class="remedy"`} {
		if !strings.Contains(section, class) {
			t.Errorf("the operator rule's section carries no %s", class)
		}
	}
}

// The report must read the catalog the RUN used, and this is the property rather than the symptom.
// Rules render from the verdicts and findings, so a report built against a narrower catalog still
// shows every rule that ran; only the prose goes missing, which is why the defect was quiet. Compare
// the two sets directly so a refactor that re-narrows the catalog fails here rather than in a reader's
// report.
func TestEveryRuleThatRanHasASectionWithProse(t *testing.T) {
	csv := runCheck(t, "--verdicts", "--format", "csv", tutorialDesign)
	ran := map[string]bool{}
	for _, line := range strings.Split(csv, "\n")[1:] {
		if f := strings.Split(line, ","); len(f) > 2 && f[2] != "" {
			ran[f[2]] = true
		}
	}
	if len(ran) == 0 {
		t.Fatal("no rule produced a verdict, so this compared nothing")
	}

	out := runCheck(t, "--verdicts", "--format", "html", tutorialDesign)
	withProse := map[string]bool{}
	for _, p := range sectionSummaries.FindAllStringSubmatch(out, -1) {
		if strings.TrimSpace(p[2]) != "" {
			withProse[p[1]] = true
		}
	}
	for name := range ran {
		if !withProse[name] {
			t.Errorf("%s produced verdicts but has no prose in the report, so the report is reading a "+
				"narrower catalog than the run did", name)
		}
	}
}
