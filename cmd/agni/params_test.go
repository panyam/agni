package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// staleSpec seeds one part whose verification was taken against a revision the corpus no longer
// holds: doc_content_hash "sha256:OLD" against the document's current "sha256:NEW". That is the
// state the command exists to surface, and the one a reader has to be able to act on.
const staleSpec = `mpn: "ACME-LDO"
manufacturer: "Acme"
device_class: "ldo"
docs {
  id: "ds"
  title: "ACME-LDO Rev B"
  vendor: "Acme"
  content_hash: "sha256:NEW"
}
parameters {
  name: "Supply voltage, recommended operating"
  symbol: "VDD"
  limit_kind: LIMIT_KIND_RECOMMENDED_OPERATING
  value { min: 3.0 typ: 3.3 max: 3.6 }
  unit: "V"
  conditions { symbol: "TA" eq: 25 unit: "C" raw: "TA = 25C" }
  condition_coverage: CONDITION_COVERAGE_COMPLETE
  prov { doc_ref: "ds" page: 6 table_or_figure: "Recommended Operating Conditions" method: "hand" confidence: 1 }
  verification { by: "sri" doc_content_hash: "sha256:OLD" at: "2026-01-05" doc_revision: "ACME-LDO Rev A" }
}
parameters {
  name: "Quiescent current"
  symbol: "IQ"
  limit_kind: LIMIT_KIND_CHARACTERISTIC
  value { typ: 0.000042 }
  unit: "A"
  condition_coverage: CONDITION_COVERAGE_UNCONDITIONAL
  prov { doc_ref: "ds" page: 8 table_or_figure: "Electrical Characteristics" method: "hand" confidence: 1 }
}
`

// seedParams writes one corpus directory and returns its path.
func seedParams(t *testing.T, dir, body string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.textproto"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func paramsOut(t *testing.T, args ...string) string {
	t.Helper()
	cmd := rootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out.String())
	}
	return out.String()
}

func paramsErr(t *testing.T, args ...string) error {
	t.Helper()
	cmd := rootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	return cmd.Execute()
}

// TestParamsRendersTheRecord: the point of the command is the part of the record the flat relations
// cannot carry, so the conditions, the pin binding column, the citation and the typical value all
// have to reach the page.
func TestParamsRendersTheRecord(t *testing.T) {
	dir := t.TempDir()
	seedParams(t, filepath.Join(dir, "p"), staleSpec)
	t.Chdir(dir)

	got := paramsOut(t, "params", "ACME-LDO", "--params", "p")
	for _, want := range []string{
		"ACME-LDO", "Acme", "ldo",
		"ACME-LDO Rev B", // the document revision the corpus holds
		"VDD", "IQ",      // both parameters
		"recommended_operating", // the limit kind, not a bare number
		"3.3",                   // the TYPICAL value, which no flat relation carried before #545
		"TA = 25C",              // the conditions the value is valid under
		"Recommended Operating Conditions",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not carry %q:\n%s", want, got)
		}
	}
}

// TestParamsNamesBothRevisionsForAStaleVerification is the assertion the command exists for.
//
// Staleness is decided by content hash and NEVER by the printed revision (DECISIONS.md, "A document
// revision is recorded for the reader, and never compared"). What a reader can act on is the pair:
// the revision that was checked, and the one the corpus holds now. Quoting two hashes is not a task.
func TestParamsNamesBothRevisionsForAStaleVerification(t *testing.T) {
	dir := t.TempDir()
	seedParams(t, filepath.Join(dir, "p"), staleSpec)
	t.Chdir(dir)

	got := paramsOut(t, "params", "ACME-LDO", "--params", "p")
	if !strings.Contains(got, "stale") {
		t.Errorf("a verification pinned to a superseded revision must report stale:\n%s", got)
	}
	if !strings.Contains(got, "ACME-LDO Rev A") {
		t.Errorf("the revision that was VERIFIED is missing, so the re-confirm task cannot be acted on:\n%s", got)
	}
	if !strings.Contains(got, "ACME-LDO Rev B") {
		t.Errorf("the revision the corpus HOLDS is missing, so the reader cannot see what to check against:\n%s", got)
	}
	// The unverified parameter must not borrow its sibling's state.
	if !strings.Contains(got, "unverified") {
		t.Errorf("the parameter carrying no verification must say so:\n%s", got)
	}
}

// TestParamsMPNMatchIsCaseInsensitiveButNotFuzzy follows param.ParamSet.Lookup: vendor and BOM
// casing of one MPN differ routinely, while a near-miss MPN is a DIFFERENT PART until a human says
// otherwise.
func TestParamsMPNMatchIsCaseInsensitiveButNotFuzzy(t *testing.T) {
	dir := t.TempDir()
	seedParams(t, filepath.Join(dir, "p"), staleSpec)
	t.Chdir(dir)

	if got := paramsOut(t, "params", "acme-ldo", "--params", "p"); !strings.Contains(got, "ACME-LDO") {
		t.Errorf("lower-cased MPN did not match:\n%s", got)
	}
	if err := paramsErr(t, "params", "ACME-LDO-2", "--params", "p"); err == nil {
		t.Error("a near-miss MPN matched; it is a different part until a human says otherwise")
	}
}

// TestParamsMissIsAnError: an MPN nothing seeds must not print an empty record, which reads as a
// part with no parameters rather than a part nobody has transcribed.
func TestParamsMissIsAnError(t *testing.T) {
	dir := t.TempDir()
	seedParams(t, filepath.Join(dir, "p"), staleSpec)
	t.Chdir(dir)

	err := paramsErr(t, "params", "NOSUCHPART", "--params", "p")
	if err == nil {
		t.Fatal("a miss returned no error")
	}
	if !strings.Contains(err.Error(), "NOSUCHPART") {
		t.Errorf("the error does not name the MPN asked for: %v", err)
	}
}

// TestParamsNeedsACorpus: with neither --params nor --design there is nothing to read, and saying so
// beats printing an empty record.
func TestParamsNeedsACorpus(t *testing.T) {
	t.Chdir(t.TempDir())
	err := paramsErr(t, "params", "ACME-LDO")
	if err == nil {
		t.Fatal("no corpus named and no error")
	}
	// Naming both routes in, since "no corpus" without them leaves the reader to guess which flag
	// they were supposed to pass.
	for _, want := range []string{"--params", "--design"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not mention %s: %v", want, err)
		}
	}
}

// TestParamsJSONIsTheRecordItself round-trips the output back through protojson. A field the
// renderer drops fails here, which a substring assertion on rendered text cannot catch.
func TestParamsJSONIsTheRecordItself(t *testing.T) {
	dir := t.TempDir()
	seedParams(t, filepath.Join(dir, "p"), staleSpec)
	t.Chdir(dir)

	got := paramsOut(t, "params", "ACME-LDO", "--params", "p", "--format", "json")
	var spec parampb.PartSpec
	if err := protojson.Unmarshal([]byte(got), &spec); err != nil {
		t.Fatalf("output is not a PartSpec: %v\n%s", err, got)
	}
	if spec.GetMpn() != "ACME-LDO" {
		t.Errorf("mpn = %q, want ACME-LDO", spec.GetMpn())
	}
	if len(spec.GetParameters()) != 2 {
		t.Fatalf("parameters = %d, want 2", len(spec.GetParameters()))
	}
	// The nested parts the flat relations cannot carry are the reason this command exists.
	if got := spec.GetParameters()[0].GetVerification().GetDocRevision(); got != "ACME-LDO Rev A" {
		t.Errorf("verification doc_revision = %q, want the snapshot", got)
	}
	if len(spec.GetParameters()[0].GetConditions()) != 1 {
		t.Error("the conditions did not survive the round trip")
	}
	if spec.GetParameters()[1].GetValue().GetTyp() == 0 {
		t.Error("the typical value did not survive the round trip")
	}
}

// TestParamsProjectCorpusWinsOverTheFlag is the agni issue 474 shape, applied here before it can
// bite: a command that reads its tier from the flag alone answers about the wrong corpus inside a
// project. The two corpora seed DIFFERENT numbers for one MPN, so reading the flag produces a
// visibly wrong answer rather than an identical one.
func TestParamsProjectCorpusWinsOverTheFlag(t *testing.T) {
	proj := t.TempDir()
	mk := func(rel, body string) {
		p := filepath.Join(proj, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	spec := func(max string) string {
		return `mpn: "ACME-SEEDED"
docs { id: "ds" title: "ACME Rev B" }
parameters {
  name: "Supply voltage"
  symbol: "VDD"
  limit_kind: LIMIT_KIND_ABSOLUTE_MAX
  value { max: ` + max + ` }
  unit: "V"
  condition_coverage: CONDITION_COVERAGE_UNCONDITIONAL
  prov { doc_ref: "ds" page: 3 table_or_figure: "Absolute Maximum Ratings" method: "hand" confidence: 1 }
}
`
	}
	mk("project.yaml", "name: board\ntitle: Test project\n")
	mk("params/seeded.textproto", spec("4.6"))
	mk("designs/board/design.yaml", "name: board\ntitle: Test board\nentry: board.edn\n")
	mk("designs/board/board.edn", mpnEDN)
	mk("other/seeded.textproto", spec("99.9"))
	t.Chdir(proj)

	got := paramsOut(t, "params", "ACME-SEEDED", "--design", "designs/board/board.edn", "--params", "other")
	if !strings.Contains(got, "4.6") {
		t.Errorf("the project's own params/ did not win over --params (Overlay.SpecsOr):\n%s", got)
	}
	if strings.Contains(got, "99.9") {
		t.Errorf("the flag's corpus answered inside a project that declares its own:\n%s", got)
	}
}

// TestParamsDesignOutsideAProjectFallsBackToTheFlag: the flag is the route for a loose design, so
// widening to the project path must not close it.
func TestParamsDesignFlagFallsBackToTheCorpusFlag(t *testing.T) {
	dir := t.TempDir()
	seedParams(t, filepath.Join(dir, "p"), staleSpec)
	if err := os.WriteFile(filepath.Join(dir, "board.edn"), []byte(mpnEDN), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	if got := paramsOut(t, "params", "ACME-LDO", "--design", "board.edn", "--params", "p"); !strings.Contains(got, "ACME-LDO Rev B") {
		t.Errorf("a design in no project must fall through to --params:\n%s", got)
	}
}
