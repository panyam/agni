package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckStrapBandCLI (WS3-119) is the end-to-end proof that a declared strap band reaches the
// rule: through the YAML loader, into the compiled intent catalog, against a real EDIF read whose
// resistor values came from the ingestion value pass rather than a hand-built IR.
//
// BOOT_MODE0 and BOOT_MODE1 are both strapped HIGH, so the direction half is satisfied on both and
// cannot be what fires. Only the VALUE separates them, which is what makes this a test of the new
// half rather than of the old one.
func TestCheckStrapBandCLI(t *testing.T) {
	cmd := checkCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"testdata/intent/strap.edn", "--intent-path", "testdata/intent/strap-band.yaml"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if !strings.Contains(out, "BOOT_MODE0") {
		t.Errorf("the 100R strap is below the declared 1k minimum and must be reported:\n%s", out)
	}
	// The finding has to be actionable: which resistor, its value as WRITTEN, and the bound it broke.
	for _, want := range []string{"R1", "100R", "1k"} {
		if !strings.Contains(out, want) {
			t.Errorf("finding is missing %q, which a reviewer needs to act on it:\n%s", want, out)
		}
	}
	// The 10k strap sits inside the same band and must not be reported.
	if strings.Contains(out, "BOOT_MODE1") {
		t.Errorf("the 10k strap is inside the declared band and must be silent:\n%s", out)
	}
}

// TestCheckStrapImpossibleBandRejected: a band nothing can satisfy fails the LOAD rather than
// producing a rule that runs and can never pass. Before this, the field was unknown to the loader and
// silently dropped, so a declaration that looked enforced was not.
func TestCheckStrapImpossibleBandRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("name: bad\nnet_properties:\n  - {net: B0, property: strap, value: high, min_ohms: 100000, max_ohms: 1000}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := checkCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"testdata/intent/strap.edn", "--intent-path", path})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("an impossible band must fail the load, not run:\n%s", out.String())
	}
	if !strings.Contains(err.Error(), "a band nothing can satisfy") {
		t.Errorf("error should say what is wrong with the declaration: %v", err)
	}
}
