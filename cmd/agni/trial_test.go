package main

import (
	"bytes"
	"context"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/panyam/agni/feedback"
)

// TestATrialReportHoldsNothingFromTheDesign is agni issue 1057's guarantee, asserted rather than
// promised: every name the tutorial design and its project carry (nets, reference designators, part
// numbers, the design's and project's names, its mount and files, its project rules) is looked for in
// the report's markdown, its JSON and its issue link, and none is there.
func TestATrialReportHoldsNothingFromTheDesign(t *testing.T) {
	freshWorkspace(t)
	root := filepath.Join("..", "..", "examples", "tutorial-project")
	cliMountSpecs = []string{"tut=" + root}
	const design = "mount://tut/designs/gateway"
	rep, err := runTrial(context.Background(), design)
	if err != nil {
		t.Fatal(err)
	}
	var js bytes.Buffer
	if err := writeTrial(&js, "json", rep); err != nil {
		t.Fatal(err)
	}
	link, err := url.QueryUnescape(rep.IssueURL(feedbackRepo))
	if err != nil {
		t.Fatal(err)
	}
	outputs := map[string]string{"markdown": rep.Markdown(), "json": js.String(), "issue link": link}

	// What the design and its project hold, read the way any client would.
	names := map[string]string{}
	for _, q := range []struct{ query, kind string }{
		{"net.pin_count(?n, ?c) => ?n", "net"},
		{"component.net(?r, ?n) => ?r", "ref-des"},
		{"component.mpn(?r, ?m) => ?m", "part number"},
	} {
		out := runCLI(t, queryCmd(), design, q.query, "--format", "csv")
		for _, line := range strings.Split(out, "\n")[1:] {
			if cell := strings.Trim(strings.SplitN(line, ",", 2)[0], `"`); cell != "" {
				names[cell] = q.kind
			}
		}
	}
	for _, s := range []string{"gateway", "Sample Board", "tutorial-project", "tut", "gateway.edn", "gateway.kicad_pcb", "gateway-profiles", "house"} {
		names[s] = "design, project, mount or file name"
	}
	// The positive control: the names are really there to find, and the project's own rules really
	// reported something, so a report that says nothing would not pass for a sanitized one.
	if n := len(names); n < 40 {
		t.Fatalf("collected %d names from the design, too few for the check to mean anything", n)
	}
	if rep.ProjectRules.Findings == 0 || len(rep.Rules) < 10 {
		t.Errorf("project findings %d, shipped rules %d; the run should report both", rep.ProjectRules.Findings, len(rep.Rules))
	}
	for name, kind := range names {
		re := regexp.MustCompile(`(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(name) + `($|[^A-Za-z0-9_])`)
		for where, text := range outputs {
			if re.MatchString(text) {
				t.Errorf("the %s carries the %s %q", where, kind, name)
			}
		}
	}
	for _, st := range rep.Steps {
		if st.Status != feedback.StatusOK {
			t.Errorf("step %q: %s (%s), want every step of a trial on the tutorial board to work", st.Step, st.Status, st.Error)
		}
	}
}

// A rule a project wrote is counted and never named, because its namespace is the project's name.
func TestOnlyAShippedRuleIsNamed(t *testing.T) {
	shipped := map[string]bool{"i2c-pull-up": true}
	if r, ok := feedback.Shipped("i2c-pull-up", shipped); !ok || r.String() != "i2c-pull-up" {
		t.Errorf("a shipped rule was refused: %v %v", r, ok)
	}
	if _, ok := feedback.Shipped("acme-profiles/can-esd-missing", shipped); ok {
		t.Error("a project's rule was accepted as a shipped one")
	}
}
