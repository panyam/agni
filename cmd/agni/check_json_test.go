package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckJSON pins the `check --format json` wire contract the rules-conformance runner reads:
// a findings array, each with rule + subject{kind,ref} + severity + message + provenance carried
// through from the reader. Uses the EDIF conformance fixture (single-pin net STUB) so the shape is
// asserted on a real reader path, not a stub.
func TestCheckJSON(t *testing.T) {
	cmd := checkCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--format", "json", "--rule", "single-pin-net", "testdata/conformance/fires.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("check --format json: %v", err)
	}

	var got struct {
		Findings []struct {
			Rule     string `json:"rule"`
			Severity string `json:"severity"`
			Subject  struct {
				Kind string `json:"kind"`
				Ref  string `json:"ref"`
			} `json:"subject"`
			Message    string `json:"message"`
			Provenance struct {
				SourceFile string `json:"sourceFile"`
			} `json:"provenance"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if len(got.Findings) != 1 {
		t.Fatalf("findings = %d, want 1\n%s", len(got.Findings), out.String())
	}
	f := got.Findings[0]
	if f.Rule != "single-pin-net" || f.Subject.Ref != "STUB" || f.Subject.Kind != "net" {
		t.Errorf("finding = %+v, want single-pin-net on net STUB", f)
	}
	if f.Severity != "info" {
		t.Errorf("severity = %q, want info", f.Severity)
	}
	if f.Provenance.SourceFile == "" {
		t.Error("provenance.sourceFile should be carried through, got empty")
	}
}

// TestCheckJSONEmpty checks that a clean design still emits a well-formed object with an empty
// findings array (never null), so a consumer parses one shape regardless of finding count.
func TestCheckJSONEmpty(t *testing.T) {
	cmd := checkCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--format", "json", "testdata/conformance/passes.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("check --format json: %v", err)
	}
	var got struct {
		Findings []json.RawMessage `json:"findings"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if len(got.Findings) != 0 {
		t.Errorf("findings = %d, want 0", len(got.Findings))
	}
}

// TestCheckConventionsFlag: --conventions composes an operator naming config into the
// catalog for one run; its rules appear under the config's namespace and fire like any
// built-in.
func TestCheckConventionsFlag(t *testing.T) {
	cmd := checkCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--format", "json", "--rule", "example/signal-net-naming",
		"--conventions", "testdata/conventions/example.yaml", "testdata/conformance/dupname.fires.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("check --conventions: %v", err)
	}
	var got struct {
		Findings []struct {
			Rule    string               `json:"rule"`
			Subject struct{ Ref string } `json:"subject"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	// The fixture's names (VCC, SIG) all conform to the example convention: compiled, ran, quiet.
	if len(got.Findings) != 0 {
		t.Fatalf("conforming names must not fire: %+v", got.Findings)
	}

	// A stricter convention fires, with the finding attributed to the namespaced rule.
	strict := filepath.Join(t.TempDir(), "strict.yaml")
	if err := os.WriteFile(strict, []byte("name: strict\nrules:\n  - name: sig-only\n    allow: [\"^SIG$\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd = checkCmd()
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--format", "json", "--rule", "strict/sig-only",
		"--conventions", strict, "testdata/conformance/dupname.fires.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("check --conventions strict: %v", err)
	}
	got.Findings = nil
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if len(got.Findings) != 2 {
		t.Fatalf("findings = %d, want the two VCC nets\n%s", len(got.Findings), out.String())
	}
	for _, f := range got.Findings {
		if f.Rule != "strict/sig-only" || f.Subject.Ref != "VCC" {
			t.Errorf("finding = %+v, want strict/sig-only on VCC", f)
		}
	}
}

// sheetFinding is the finding subset the sheet-annotation tests read (WS3-023): the subject and
// the sheets it locates on. Shared by the JSON and report acceptance tests.
type sheetFinding struct {
	Subject struct {
		Kind string `json:"kind"`
		Ref  string `json:"ref"`
	} `json:"subject"`
	Sheets []string `json:"sheets"`
}

// TestCheckJSONSheets pins WS3-023 + WS9-048: `check --format json` carries each finding's sheet
// membership. On the multi-sheet sheetnav fixture the U1 duplicate-ref-des finding (a component
// subject) locates on the root sheet via faithful geometry. A bare netlist has no faithful sheets,
// so it renders via auto-layout and its findings locate on the single auto-layout sheet "graph" —
// the same place the web viewer draws them, now that the CLI shares the service's layout-based
// annotation (before the thin-client conversion the CLI used faithful-only geometry and these were
// empty).
func TestCheckJSONSheets(t *testing.T) {
	parse := func(path string) []sheetFinding {
		cmd := checkCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--format", "json", path})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("check --format json %s: %v", path, err)
		}
		var got struct {
			Findings []sheetFinding `json:"findings"`
		}
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, out.String())
		}
		return got.Findings
	}

	var u1 []string
	var found bool
	for _, f := range parse("testdata/conformance/sheetnav.fires.kicad_sch") {
		if f.Subject.Kind == "component" && f.Subject.Ref == "U1" {
			u1, found = f.Sheets, true
		}
	}
	if !found {
		t.Fatal("expected a U1 component finding on sheetnav")
	}
	if len(u1) != 1 || u1[0] != "/" {
		t.Errorf("U1 sheets = %v, want [\"/\"]", u1)
	}

	for _, f := range parse("testdata/conformance/diffpair.fires.edn") {
		if len(f.Sheets) != 1 || f.Sheets[0] != "graph" {
			t.Errorf("%s/%s: netlist finding should locate on the auto-layout sheet [graph], got %v", f.Subject.Kind, f.Subject.Ref, f.Sheets)
		}
	}
}

// TestRegulatorOutputAbsMaxConformance is WS3-028's end-to-end acceptance: the connection-aware rule
// running through the CLI on a committed fixture, with BOTH datasheets on the wire. The unit tests
// cover the comparison; this covers the whole path, including that the plural citation field survives
// the proto round trip that a unit test never exercises.
func TestRegulatorOutputAbsMaxConformance(t *testing.T) {
	cmd := checkCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--format", "json", "--rule", "regulator-output-exceeds-abs-max",
		"--params", "testdata/conformance/regparams", "testdata/conformance/regout.fires.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("check: %v", err)
	}
	var doc struct {
		Findings []struct {
			Subject struct {
				Ref string `json:"ref"`
			} `json:"subject"`
			Message    string `json:"message"`
			Datasheets []struct {
				Doc string `json:"doc"`
			} `json:"datasheets"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out.String())
	}
	if len(doc.Findings) != 1 {
		t.Fatalf("want 1 finding, got %d: %s", len(doc.Findings), out.String())
	}
	f := doc.Findings[0]
	if f.Subject.Ref != "U2" {
		t.Errorf("subject = %q, want U2 (the endangered part)", f.Subject.Ref)
	}
	// Two citations, one per part the conclusion rests on. A single-citation regression here would
	// still produce a correct-looking finding, which is why the count is asserted and not just the docs.
	if len(f.Datasheets) != 2 {
		t.Fatalf("want 2 citations on the wire, got %d: %+v", len(f.Datasheets), f.Datasheets)
	}
	if f.Datasheets[0].Doc != "ACME-33 Rev B" || f.Datasheets[1].Doc != "ACME-REG Rev A" {
		t.Errorf("citations = %+v, want the load's doc then the source's", f.Datasheets)
	}
}

// TestFetVdssConformance is WS3-116's end-to-end, and it runs against the REAL seeded BSS138 rather
// than a synthetic fixture spec: a 50V part on a 60V rail. Unlike the WS3-028 conformance test, the
// rail's voltage here comes from the net NAME, so exactly one citation should ride the wire — the
// FET's. A regression that cited a nonexistent second source would still look plausible in the
// message, so the count is the assertion that catches it.
func TestFetVdssConformance(t *testing.T) {
	cmd := checkCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--format", "json", "--rule", "fet-vdss-below-switched-rail",
		"--params", "testdata/conformance/fetparams", "testdata/conformance/fetvdss.fires.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("check: %v", err)
	}
	var doc struct {
		Findings []struct {
			Subject struct {
				Ref string `json:"ref"`
			} `json:"subject"`
			Message    string `json:"message"`
			Datasheets []struct {
				Doc string `json:"doc"`
			} `json:"datasheets"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out.String())
	}
	if len(doc.Findings) != 1 {
		t.Fatalf("want 1 finding, got %d: %s", len(doc.Findings), out.String())
	}
	f := doc.Findings[0]
	if f.Subject.Ref != "Q1" {
		t.Errorf("subject = %q, want Q1", f.Subject.Ref)
	}
	if !strings.Contains(f.Message, "from the net name") {
		t.Errorf("message must record that the rail voltage was name-derived: %s", f.Message)
	}
	if len(f.Datasheets) != 1 {
		t.Fatalf("a name-derived rail earns no citation: want 1, got %d: %+v", len(f.Datasheets), f.Datasheets)
	}
}
