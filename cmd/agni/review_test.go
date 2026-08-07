package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestReviewCmd runs a review manifest over a broken-CAN design and checks the per-item outcomes end
// to end: the built-in CAN profile fails the termination + signal items, an unbound item reads
// not-automated, a shipped datasheet rule with no --params seeded and a board rule on a netlist both
// read not-applicable, and an inline house-rule query passes.
func TestReviewCmd(t *testing.T) {
	cmd := reviewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--checklist", "testdata/review/mini.yaml", "testdata/review/can-broken.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("review: %v", err)
	}
	s := out.String()
	for _, want := range []string{
		"1 pass, 2 fail, 2 n/a, 1 not-automated (of 6)",
		"| 202 | termination strategy | fail |",
		"| 198 | bus signals present | fail |",
		"| 196 | transceiver selection | not-automated | manual review — needs the design-intent contract |",
		"| 197 | voltage compatibility | not-applicable | needs a seeded datasheet parameter set (check --params)",
		"| b1 | track widths | not-applicable | design carries no board geometry",
		"| h1 | no forbidden MPN | pass |",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("report missing %q\n---\n%s", want, s)
		}
	}
}

// TestReviewCmdBoardPath: --board-path attaches a SEPARATE board-geometry file to the netlist review,
// so the board-tier DRC item that reads not-applicable without it (WS3-008 rules gated by
// check.Available) flips to a genuine pass/fail (WS3-089). The fires board routes a sub-floor trace so
// track-width fails; the passes board is clean so the same item passes.
func TestReviewCmdBoardPath(t *testing.T) {
	run := func(board string) string {
		cmd := reviewCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--checklist", "testdata/review/mini.yaml",
			"--board-path", board, "testdata/review/can-broken.edn"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("review --board-path %s: %v", board, err)
		}
		return out.String()
	}
	if s := run("testdata/conformance/drc.fires.kicad_pcb"); !strings.Contains(s, "| b1 | track widths | fail |") {
		t.Errorf("fires board: item b1 not a fail\n---\n%s", s)
	}
	if s := run("testdata/conformance/drc.passes.kicad_pcb"); !strings.Contains(s, "| b1 | track widths | pass |") {
		t.Errorf("passes board: item b1 not a pass\n---\n%s", s)
	}
}

// TestReviewCmdBoardPathNonBoard: --board-path at a file whose format carries no board geometry (a
// netlist) is a loud error, never a silent no-op — an explicit board request that reads nothing would
// otherwise report the board items as clean without ever checking them.
func TestReviewCmdBoardPathNonBoard(t *testing.T) {
	cmd := reviewCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"--checklist", "testdata/review/mini.yaml",
		"--board-path", "testdata/review/can-broken.edn", "testdata/review/can-broken.edn"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("--board-path at a netlist file did not error")
	}
}

// TestReviewCmdMultiDesign: several designs produce a project rollup — a manifest-level automation
// header (stated ONCE, not summed across designs), a per-design outcome summary, and a per-item x
// design traceability matrix. The same item reads its own outcome per design (202 fails on the broken
// design, passes on the other) and a not-automated item stays not-automated in every column.
func TestReviewCmdMultiDesign(t *testing.T) {
	cmd := reviewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--checklist", "testdata/review/mini.yaml",
		"testdata/review/can-broken.edn", "testdata/profiles/overlay-bus.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("review multi: %v", err)
	}
	s := out.String()
	for _, want := range []string{
		"# Review rollup: Mini ECU review",
		"**5 of 6 items covered** (manifest-level), 1 not-automated.", // coverage stated once, not 10 of 12
		"## Per-design outcomes",
		"| `testdata/review/can-broken.edn` | 1 | 2 | 0 | 0 | 0 | 0 | 2 |",
		"| `testdata/profiles/overlay-bus.edn` | 2 | 0 | 0 | 0 | 0 | 0 | 3 |",
		"## Traceability matrix",
		"### CAN Interface",
		"| 202 | termination strategy | fail | pass |",                    // per-design outcome, same item
		"| 196 | transceiver selection | not-automated | not-automated |", // not-automated in every column
	} {
		if !strings.Contains(s, want) {
			t.Errorf("rollup missing %q\n---\n%s", want, s)
		}
	}
	if strings.Contains(s, "12 items automated") || strings.Contains(s, "10 of 12") {
		t.Error("automation must be manifest-level (stated once), never summed across designs")
	}
}

// Multi-design --coverage is the summary rollup only (per-design outcomes), no per-item matrix.
func TestReviewCmdMultiCoverage(t *testing.T) {
	cmd := reviewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--coverage", "--checklist", "testdata/review/mini.yaml",
		"testdata/review/can-broken.edn", "testdata/profiles/overlay-bus.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("review multi --coverage: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "## Per-design outcomes") {
		t.Errorf("coverage rollup missing the per-design summary\n%s", s)
	}
	if strings.Contains(s, "## Traceability matrix") {
		t.Error("--coverage is the summary only; it must not emit the per-item matrix")
	}
}

// Multi-design --format json carries the manifest-level automation counts, a per-design summary, and
// the per-item outcome-by-design matrix.
func TestReviewCmdMultiJSON(t *testing.T) {
	cmd := reviewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--format", "json", "--checklist", "testdata/review/mini.yaml",
		"testdata/review/can-broken.edn", "testdata/profiles/overlay-bus.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("review multi --format json: %v", err)
	}
	var got struct {
		Total        int                       `json:"total"`
		Automated    int                       `json:"automated"`
		NotAutomated int                       `json:"not_automated"`
		PerDesign    []struct{ Design string } `json:"per_design"`
		Areas        []struct {
			Items []struct {
				ID       string
				Outcomes map[string]string
			}
		}
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("json: %v", err)
	}
	if got.Total != 6 || got.Automated != 5 || got.NotAutomated != 1 {
		t.Errorf("automation counts = {total:%d automated:%d not_automated:%d}, want {6 5 1}", got.Total, got.Automated, got.NotAutomated)
	}
	if len(got.PerDesign) != 2 {
		t.Errorf("per_design = %d designs, want 2", len(got.PerDesign))
	}
	// item 202's outcome differs by design
	var found bool
	for _, a := range got.Areas {
		for _, it := range a.Items {
			if it.ID == "202" {
				found = true
				if it.Outcomes["testdata/review/can-broken.edn"] != "fail" {
					t.Errorf("item 202 on can-broken = %q, want fail", it.Outcomes["testdata/review/can-broken.edn"])
				}
			}
		}
	}
	if !found {
		t.Error("item 202 missing from the matrix")
	}
}

// --coverage emits the per-area rollup instead of the per-item report.
func TestReviewCmdCoverage(t *testing.T) {
	cmd := reviewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--coverage", "--checklist", "testdata/review/mini.yaml", "testdata/review/can-broken.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("review --coverage: %v", err)
	}
	s := out.String()
	for _, want := range []string{
		"# Review coverage: Mini ECU review",
		"**5 of 6 covered**",
		"| CAN Interface | 3/4 | 0 | 2 | 0 | 0 | 0 | 0 | 1 | 1 |",
		"| **Total** | 5/6 | 1 | 2 | 0 | 0 | 0 | 0 | 2 | 1 |",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("coverage report missing %q\n%s", want, s)
		}
	}
}

// A profile item whose interface is absent from the design reads not-applicable, not a silent pass
// (WS3-051). The overlay-bus fixture has no CAN, so the CAN items go n/a.
func TestReviewCmdNotApplicable(t *testing.T) {
	cmd := reviewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--checklist", "testdata/review/mini.yaml", "testdata/profiles/overlay-bus.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("review: %v", err)
	}
	if s := out.String(); !strings.Contains(s, "| 198 | bus signals present | not-applicable | interface not present on this design |") {
		t.Errorf("CAN item on a no-CAN design should be not-applicable\n%s", s)
	}
}

// --format json emits the full report as JSON, with the findings that failed each item. It is the
// tooling surface (the markdown Detail cell caps findings; JSON does not).
func TestReviewCmdFormatJSON(t *testing.T) {
	cmd := reviewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--format", "json", "--checklist", "testdata/review/mini.yaml", "testdata/review/can-broken.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("review --format json: %v", err)
	}
	var rep struct {
		Areas []struct {
			Items []struct {
				ID       string `json:"id"`
				Outcome  string `json:"outcome"`
				Findings []struct {
					Rule    string `json:"rule"`
					Subject string `json:"subject"`
				} `json:"findings"`
			} `json:"items"`
		} `json:"areas"`
	}
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	var fails int
	for _, a := range rep.Areas {
		for _, it := range a.Items {
			if it.Outcome == "fail" {
				fails++
				if len(it.Findings) == 0 {
					t.Errorf("failing item %q has no findings in JSON", it.ID)
				}
			}
		}
	}
	if fails != 2 {
		t.Errorf("want 2 failing items in JSON, got %d\n%s", fails, out.String())
	}
}

// runReview runs the review command and returns its stdout, failing the test on error.
func runReview(t *testing.T, args ...string) string {
	t.Helper()
	cmd := reviewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("review %v: %v", args, err)
	}
	return out.String()
}

// TestReviewIntentContract is the end-to-end proof of the design-intent contract: an intent-bound item
// reads not-automated with no --intent-path (the honest default — no declaration, nothing to check
// against), flips to fail when the loaded design DEVIATES from a declaration (a missing module, an
// absent rail, an incomplete subsystem, an unprotected rail), and passes when the design matches. It is the load-bearing
// guard: the outcome is driven by the external declaration, not derived from the netlist.
func TestReviewIntentContract(t *testing.T) {
	checklist := "testdata/intent/checklist.yaml"
	design := "testdata/intent/ecu.edn"

	// No --intent-path: the intent rules are not in the catalog, but the items BIND an intent/ rule, so
	// they read needs-design-intent (covered, blocked on a declaration), not the misleading not-automated
	// (WS10-014). Supplying --intent-path flips them to pass/fail (below).
	base := runReview(t, "--checklist", checklist, design)
	for _, want := range []string{
		"| 4 | component counts match architecture | needs-design-intent |",
		"| 5 | all required modules present | needs-design-intent |",
		"| 7 | voltage domains identified | needs-design-intent |",
		"| 8 | power tree consistency | needs-design-intent |",
		"| 30 | rail discharge | needs-design-intent |",
	} {
		if !strings.Contains(base, want) {
			t.Errorf("without --intent-path, report missing %q\n%s", want, base)
		}
	}

	// Deviating declaration: the design has two ics (intent expects one), no soc, and no 1V8 rail, so
	// the count, module, domain, and subsystem items all fail.
	dev := runReview(t, "--checklist", checklist, "--intent-path", "testdata/intent/deviating.yaml", design)
	for _, want := range []string{
		"| 4 | component counts match architecture | fail |",
		"| 5 | all required modules present | fail |",
		"| 7 | voltage domains identified | fail |",
		"| 8 | power tree consistency | fail |",
		"| 30 | rail discharge | fail |",
	} {
		if !strings.Contains(dev, want) {
			t.Errorf("deviating intent should fail, report missing %q\n%s", want, dev)
		}
	}

	// Matching declaration: exactly two ics are present and 3V3 sits at 3.3V, so the items pass.
	ok := runReview(t, "--checklist", checklist, "--intent-path", "testdata/intent/matching.yaml", design)
	for _, want := range []string{
		"| 4 | component counts match architecture | pass |",
		"| 5 | all required modules present | pass |",
		"| 7 | voltage domains identified | pass |",
		"| 8 | power tree consistency | pass |",
		"| 30 | rail discharge | pass |",
	} {
		if !strings.Contains(ok, want) {
			t.Errorf("matching intent should pass, report missing %q\n%s", want, ok)
		}
	}
}

func TestReviewCmdRequiresChecklist(t *testing.T) {
	cmd := reviewCmd()
	cmd.SetArgs([]string{"testdata/review/can-broken.edn"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("review without --checklist must error")
	}
}

// runConvReview runs the conventions checklist, with or without the config, and returns the report.
func runConvReview(t *testing.T, conventions string) string {
	t.Helper()
	cmd := reviewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	args := []string{"--checklist", "testdata/review/conv.yaml"}
	if conventions != "" {
		args = append(args, "--conventions", conventions)
	}
	cmd.SetArgs(append(args, "testdata/review/conv-demo.edn"))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("review (--conventions %q): %v", conventions, err)
	}
	return out.String()
}

// TestReviewCmdConventionsRules covers the half that is merely invisible: a checklist item bound to a
// convention rule. With no config the binding resolves to zero catalog rules, which the review reports
// as not-automated — the same word it uses for a rule nobody has written, with no error to tell the
// two apart. Loading the config compiles the rule and the item reaches a real verdict.
func TestReviewCmdConventionsRules(t *testing.T) {
	if want := "| 16 | nets named consistently | not-automated |"; !strings.Contains(runConvReview(t, ""), want) {
		t.Errorf("without --conventions the item must read not-automated, missing %q", want)
	}
	got := runConvReview(t, "testdata/review/conventions.yaml")
	for _, want := range []string{
		"| 16 | nets named consistently | fail |",
		"house/signal-net-naming: lowercase_net",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("with --conventions the item must fail on the real violation, missing %q\n---\n%s", want, got)
		}
	}
}

// TestReviewCmdConventionsLexicon covers the damaging half: the vocabulary that changes what OTHER
// rules can see. This project names rails function-first (PMIC_VDD_LPM_1V8), which the start-anchored
// built-in vocabulary matches nowhere, so without the config the item passes while checking nothing.
func TestReviewCmdConventionsLexicon(t *testing.T) {
	if want := "| 63 | rails recognized | pass |"; !strings.Contains(runConvReview(t, ""), want) {
		t.Errorf("without --conventions the rail is unrecognized and the item passes hollowly, missing %q", want)
	}
	got := runConvReview(t, "testdata/review/conventions.yaml")
	if want := "PMIC_VDD_LPM_1V8 is a recognized supply rail"; !strings.Contains(got, want) {
		t.Errorf("the project rail vocabulary must reach the run; missing %q\n---\n%s", want, got)
	}
}

// TestReviewCmdConventionsReachesTheRead isolates the half that ONLY the design read can do. A net
// role has a name-match fallback on the model, so a rail item answers even if the lexicon arrives
// after ingestion; a pin's ELECTRICAL TYPE has no such fallback. EDIF's port grammar carries only
// INPUT/OUTPUT/INOUT, so a supply pin arrives under-typed and nothing but the ingestion stamp can
// promote it to power_in. If the lexicon ever stops reaching the read, this is the test that fails.
func TestReviewCmdConventionsReachesTheRead(t *testing.T) {
	if want := "| 64 | supply pins typed | pass |"; !strings.Contains(runConvReview(t, ""), want) {
		t.Errorf("without --conventions the supply pin stays under-typed, missing %q", want)
	}
	got := runConvReview(t, "testdata/review/conventions.yaml")
	if want := "U1 has a typed supply pin"; !strings.Contains(got, want) {
		t.Errorf("the lexicon must reach INGESTION for the pin to be promoted; missing %q\n---\n%s", want, got)
	}
}

// TestReviewCmdConventionsUnreadable: a named config that cannot be read fails the run. Falling back
// to the built-in vocabulary would report the design clean against conventions never applied.
func TestReviewCmdConventionsUnreadable(t *testing.T) {
	for _, path := range []string{"testdata/review/does-not-exist.yaml", "testdata/review/conv-demo.edn"} {
		cmd := reviewCmd()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetArgs([]string{"--checklist", "testdata/review/conv.yaml", "--conventions", path, "testdata/review/conv-demo.edn"})
		if err := cmd.Execute(); err == nil {
			t.Errorf("--conventions %s must error, not silently run with the defaults", path)
		}
	}
}

// runFullOverlayReview runs the conventions checklist with every overlay tier attached, optionally
// omitting the convention, and returns the report.
func runFullOverlayReview(t *testing.T, conventions string) string {
	t.Helper()
	cmd := reviewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	args := []string{
		"--checklist", "testdata/review/conv.yaml",
		"--intent-path", "testdata/review/intent.yaml",
		"--profile-path", "testdata/review/profiles",
	}
	if conventions != "" {
		args = append(args, "--conventions", conventions)
	}
	cmd.SetArgs(append(args, "testdata/review/conv-demo.edn"))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("review (--conventions %q): %v", conventions, err)
	}
	return out.String()
}

// TestReviewOverlayTiersCoexist pins that a convention carrying RULES adds to the catalog rather than
// replacing it (WS3-107).
//
// Composing the per-request overlay used to rebuild the catalog from the standard sources, which keeps
// the built-ins and every RegisterSource'd suite — true, and exactly what made the bug invisible — but
// drops the catalog it was handed. For a review that catalog is the --profile-path and --intent-path
// tiers, so adding one naming rule to a working config silently disabled interface profiles and design
// intent for the whole run: 19 items pass -> needs-design-intent and 16 pass -> not-automated on one
// real design, with no error anywhere to explain it.
//
// The assertion is that the intent-bound and profile-bound items reach a REAL verdict with the
// convention loaded. Both fixtures are authored to FAIL rather than pass, because a pass is also what
// a vanished tier could produce on a design with nothing wrong.
func TestReviewOverlayTiersCoexist(t *testing.T) {
	got := runFullOverlayReview(t, "testdata/review/conventions.yaml")
	for _, want := range []string{
		"| 16 | nets named consistently | fail |",                    // the convention's own rule
		"| 70 | rail voltage matches the declared domain | fail |",   // --intent-path survived
		"| 71 | SIGBUS signals present | fail |",                     // --profile-path survived
		"intent/voltage-domain-mismatch",
		"profile-overlay/sigbus-signal-missing",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("a convention with rules dropped an overlay tier; missing %q\n---\n%s", want, got)
		}
	}
	// Matched as a table CELL, not as a bare substring: the per-area tally lines name every outcome
	// ("0 pass, 1 fail, 0 n/a, 0 not-automated"), so a substring test here would always fire.
	for _, never := range []string{"| needs-design-intent |", "| not-automated |"} {
		if strings.Contains(got, never) {
			t.Errorf("an overlay tier went missing (%q in the report)\n---\n%s", never, got)
		}
	}
}

// TestReviewOverlayTiersCoexistWithoutAConvention is the control. The bug only triggered when the
// convention carried RULES — a lexicon-only config composes no sources and took an early return that
// was always correct — so without a convention at all, the same two items must already resolve. If
// this ever fails, the fixtures stopped exercising the tiers and the test above proves nothing.
func TestReviewOverlayTiersCoexistWithoutAConvention(t *testing.T) {
	got := runFullOverlayReview(t, "")
	for _, want := range []string{
		"| 70 | rail voltage matches the declared domain | fail |",
		"| 71 | SIGBUS signals present | fail |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the overlay tiers must resolve with no convention loaded; missing %q\n---\n%s", want, got)
		}
	}
}
