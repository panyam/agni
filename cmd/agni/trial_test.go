package main

import (
	"bytes"
	"context"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
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

// A trial under --flatten-hierarchy reads every block of a hierarchical netlist, and counts what a
// flattened read of the design holds rather than the top cell alone (agni issue 1103).
func TestATrialReadsEveryBlockUnderFlattenHierarchy(t *testing.T) {
	for _, path := range []string{
		"../../readers/edif/testdata/hier_blocks.edn",
		"testdata/github-issues/1085-hierarchical-edif/hier_two_subcells.edn",
	} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			freshWorkspace(t)
			prev := readFlatten
			t.Cleanup(func() { readFlatten = prev })
			uri, err := cliArgURI(path)
			if err != nil {
				t.Fatal(err)
			}
			trial := func(flatten bool) feedback.Shape {
				readFlatten = flatten
				rep, err := runTrial(context.Background(), uri)
				if err != nil {
					t.Fatal(err)
				}
				if st := rep.Steps[0]; st.Status != feedback.StatusOK {
					t.Fatalf("read step: %s (%s)", st.Status, st.Error)
				}
				return rep.Shape
			}

			top := trial(false)
			if top.UnexpandedBlocks == 0 || top.Flattened {
				t.Fatalf("without the flag the read left %d blocks out, flattened %v; the fixture should be hierarchical and read top-cell only", top.UnexpandedBlocks, top.Flattened)
			}
			whole := trial(true)
			stats := flattenedStats(t, path)
			if whole.UnexpandedBlocks != 0 || !whole.Flattened {
				t.Errorf("with the flag the read left %d blocks out, flattened %v; want 0 and true", whole.UnexpandedBlocks, whole.Flattened)
			}
			if whole.Components != stats.Components || whole.Nets != stats.Nets {
				t.Errorf("trial counted %d components and %d nets, a flattened read %d and %d", whole.Components, whole.Nets, stats.Components, stats.Nets)
			}
			if whole.Components <= top.Components {
				t.Errorf("flattened %d components against the top cell's %d; flattening should read more", whole.Components, top.Components)
			}
			if md := (feedback.Report{Shape: whole}).Markdown(); !strings.Contains(md, "hierarchy flattened") {
				t.Errorf("report line %q does not say the read was flattened", md)
			}
		})
	}
}

// flattenedStats is what `agni stats --flatten-hierarchy` counts for path.
func flattenedStats(t *testing.T, path string) feedback.Shape {
	t.Helper()
	readFlatten = true
	out := runCLI(t, statsCmd(), path)
	count := func(label string) int {
		m := regexp.MustCompile(label + `:\s+(\d+)`).FindStringSubmatch(out)
		if m == nil {
			t.Fatalf("stats has no %s line:\n%s", label, out)
		}
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return feedback.Shape{Components: count("components"), Nets: count("nets")}
}
