package main

import (
	"bytes"
	"strings"
	"testing"
)

// runCheck drives a fresh check command and returns its output, so each case starts from clean flag
// state rather than inheriting the previous one's.
func runCheck(t *testing.T, args ...string) string {
	t.Helper()
	cmd := checkCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("check %v: %v\n%s", args, err, out.String())
	}
	return out.String()
}

// The verdict table's whole reason for existing is that a PASS appears in it. A findings table cannot
// show one, so an engineer reading it cannot tell a clean subject from one nobody checked.
func TestVerdictCSVShowsAPassWithItsProof(t *testing.T) {
	out := runCheck(t, "--verdicts", "--format", "csv", "testdata/conformance/showcase.fires.kicad_sch")

	rows := strings.Split(strings.TrimSpace(out), "\n")
	if len(rows) < 3 {
		t.Fatalf("want a header and both subjects, got:\n%s", out)
	}
	if got := rows[0]; got != strings.Join(verdictCSVColumns, ",") {
		t.Errorf("header must be the fixed column set\n got: %s\nwant: %s", got, strings.Join(verdictCSVColumns, ","))
	}
	// Select the row this test makes claims about, NOT whichever pass happens to sort last. The
	// original loop kept the final passing row, which was this one only while i2c-pull-up was the
	// single converted rule; the first structural conversions put an unconnected-component pass
	// after it and the assertions below started reading a row they were never about. Every further
	// conversion under agni issue 391 would have re-broken a positional pick.
	var pass string
	var anyPass bool
	for _, r := range rows[1:] {
		if !strings.Contains(r, ",pass,") {
			continue
		}
		anyPass = true
		if strings.HasPrefix(r, "i2c-pull-up:net:SDA,") {
			pass = r
		}
	}
	if !anyPass {
		t.Fatalf("no passing row; a table that only shows failures is the findings table\n%s", out)
	}
	if pass == "" {
		t.Fatalf("no passing row for i2c-pull-up on SDA, the subject this test proves out\n%s", out)
	}
	// The proof, and the entities a viewer would highlight from it.
	for _, want := range []string{"i2c-pull-up:net:SDA", "SDA reaches rail +3V3 through R1", "pull-up=R1|rail=+3V3"} {
		if !strings.Contains(pass, want) {
			t.Errorf("passing row must carry %q, got: %s", want, pass)
		}
	}
}

// THE REGRESSION GUARD. Adding a second table must not change the first one, or every consumer
// binding to `check`'s output pays for a feature they did not ask for.
func TestVerdictsDoNotChangeTheFindingsOutput(t *testing.T) {
	design := "testdata/conformance/showcase.fires.kicad_sch"

	recs, raw := runCheckCSV(t, design)
	if strings.Contains(raw, "verdict_id") {
		t.Errorf("the findings csv must keep its own header:\n%s", raw)
	}
	// Every findings row is a violation, so every row carries a severity. A verdict row would not:
	// a pass has nothing to grade. Asserted on the PARSED column rather than by searching the text,
	// because a subject can legitimately appear here from another rule (SDA is reported by
	// reverse-blocking-absent on this fixture) and a substring match cannot tell the two apart.
	sev, rule, subj := 0, 2, 4
	for _, r := range recs[1:] {
		if r[sev] == "" {
			t.Errorf("a row with no severity is not a finding: %v", r)
		}
		if r[rule] == "i2c-pull-up" && r[subj] == "SDA" {
			t.Errorf("SDA passes i2c-pull-up and must not be a finding: %v", r)
		}
	}
	// The default JSON keeps its findings. It does gain an empty "verdicts" key, which EmitUnpopulated
	// makes unavoidable once the field exists on the response, so what is asserted is that no verdict
	// DATA rides along.
	//
	// Whitespace-insensitive on purpose: protojson deliberately varies the spacing after a colon to
	// discourage byte-comparing its output, so `"verdicts": []` and `"verdicts":  []` are both it.
	js := strings.Join(strings.Fields(runCheck(t, "--format", "json", design)), " ")
	if !strings.Contains(js, `"verdicts": []`) {
		t.Errorf("expected an empty verdicts key, got:\n%s", js)
	}
	if strings.Contains(js, "i2c-pull-up:net:") {
		t.Error("verdict data must not ride along in the default json")
	}
}

// A verdict id has to survive the trip out to the CLI, since it is the whole basis of a row being
// addressable from a report or a link.
func TestVerdictCSVCarriesTheDerivedID(t *testing.T) {
	out := runCheck(t, "--verdicts", "--format", "csv", "testdata/conformance/showcase.fires.kicad_sch")
	for _, want := range []string{"i2c-pull-up:net:SDA", "i2c-pull-up:net:SCL"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing verdict id %q in:\n%s", want, out)
		}
	}
}
