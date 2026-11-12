package main

import (
	"context"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/readers/formats"
)

// TestExtensionComposesReaderAndRule is the WS12-001 acceptance test. A SEPARATE module, this one,
// contributes a format reader and a rule suite through the engine's public extension points,
// and both take effect on the engine's own surfaces. The blank imports in main.go run the
// extension packages' init, so by the time this test runs the .acme reader and the acme/ rule
// are registered.
func TestExtensionComposesReaderAndRule(t *testing.T) {
	// The reader reached the engine's registry (WS12-003), so the extension resolves and loads.
	if formats.ByExt("x.acme") == nil {
		t.Fatal(".acme reader not registered with the engine's formats registry")
	}
	d, err := (&formats.Loader{}).ReadDesign("testdata/example.acme")
	if err != nil {
		t.Fatalf("loading the .acme fixture through the engine Loader: %v", err)
	}
	if len(d.Components) != 4 {
		t.Fatalf("components = %d, want 4 (R1 C1 U1 X1)", len(d.Components))
	}

	// The CLI and serve wire up one catalog, and the rule reached it (WS12-004), namespaced and
	// source-tagged.
	r := check.DefaultCatalog().Lookup("acme/no-experimental-refdes")
	if r == nil {
		t.Fatal("acme/no-experimental-refdes not in DefaultCatalog")
	}
	if r.Tags[check.KeySource] != "acme" {
		t.Errorf("source tag = %q, want acme", r.Tags[check.KeySource])
	}

	// End to end: the extension's rule runs over the extension's format and fires on X1.
	findings := check.RunBackground(check.NewModel(d), check.DefaultCatalog().Rules())
	var acme []check.Finding
	for _, f := range findings {
		if f.Rule == "acme/no-experimental-refdes" {
			acme = append(acme, f)
		}
	}
	if len(acme) != 1 || check.EntityRef(acme[0].Subject) != "X1" {
		t.Errorf("acme rule findings = %+v, want one on X1", acme)
	}
}

// TestExtensionDatalogPinRule is the WS3-038 acceptance. A separate module authors a PIN-level rule as
// DATALOG over the engine's public relations, with no engine change, and it produces findings like
// any built-in. The Go rule above proves the registration hook. This test proves the AUTHORING
// path, and the open-core story depends on that path.
func TestExtensionDatalogPinRule(t *testing.T) {
	d, err := (&formats.Loader{}).ReadDesign("testdata/example.acme")
	if err != nil {
		t.Fatal(err)
	}
	m := check.NewModel(d)

	// The extension's own reader has to declare PART-TYPE pins for any of this to work, because the
	// engine's pin relations project from declared pins, not from net connections. A format that
	// emits only connections leaves them all empty and a pin rule silently finds nothing, so this
	// guards the reader half from regressing and taking the rule's evidence with it.
	if len(m.Pins()) == 0 {
		t.Fatal("the .acme reader declared no part-type pins, so every pin relation is empty")
	}

	r := check.DefaultCatalog().Lookup("acme/experimental-on-power-net")
	if r == nil {
		t.Fatal("acme/experimental-on-power-net not in DefaultCatalog")
	}
	if r.Tags[check.KeySource] != "acme" {
		t.Errorf("source tag = %q, want acme", r.Tags[check.KeySource])
	}

	var got []string
	// Findings, not Eval. Eval returns VERDICTS, and Verdict shares Subject/Kind/Pin/NetID with
	// Finding, so `for _, f := range r.Eval(context.Background(), m)` reading f.Subject compiles and silently counts
	// passes as failures. It happens to be right today only because a datalog rule is still wrapped
	// in FailuresOnly and emits nothing but failures. It would start lying the moment a datalog
	// rule states a considered set.
	for _, f := range r.Findings(context.Background(), m) {
		got = append(got, check.EntityRef(f.Subject))
	}
	// VCC carries U1's declared VDD power pin and X1. GND carries only ground-role pins, so the
	// rule must leave it alone. A net-level rule could not make that pin-ROLE discrimination, so
	// this had to be a pin rule.
	if len(got) != 1 || got[0] != "VCC" {
		t.Errorf("datalog pin rule findings = %v, want exactly [VCC]", got)
	}
}
