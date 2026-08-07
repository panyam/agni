package main

import (
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/readers/formats"
)

// TestOverlayComposesReaderAndRule is the WS12-001 acceptance: a SEPARATE module (this one)
// contributes a format reader and a rule suite through the engine's public extension points,
// and both take effect on the engine's own surfaces. The blank imports in main.go run the
// overlay packages' init, so by the time this test runs the .acme reader and the acme/ rule
// are registered.
func TestOverlayComposesReaderAndRule(t *testing.T) {
	// The reader reached the engine's registry (WS12-003): the extension resolves and loads.
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

	// The rule reached the catalog the CLI and serve wire (WS12-004), namespaced + source-tagged.
	r := check.DefaultCatalog().Lookup("acme/no-experimental-refdes")
	if r == nil {
		t.Fatal("acme/no-experimental-refdes not in DefaultCatalog")
	}
	if r.Tags[check.KeySource] != "acme" {
		t.Errorf("source tag = %q, want acme", r.Tags[check.KeySource])
	}

	// End to end: the overlay's rule runs over the overlay's format and fires on X1.
	findings := check.Run(check.NewModel(d), check.DefaultCatalog().Rules())
	var acme []check.Finding
	for _, f := range findings {
		if f.Rule == "acme/no-experimental-refdes" {
			acme = append(acme, f)
		}
	}
	if len(acme) != 1 || acme[0].Subject != "X1" {
		t.Errorf("acme rule findings = %+v, want one on X1", acme)
	}
}

// TestOverlayDatalogPinRule is the WS3-038 acceptance: a separate module authors a PIN-level rule as
// DATALOG over the engine's public relations, with no engine change, and it produces findings like
// any built-in. The Go rule above proves the registration seam; this proves the AUTHORING seam, which
// is the one the open-core story actually rests on.
func TestOverlayDatalogPinRule(t *testing.T) {
	d, err := (&formats.Loader{}).ReadDesign("testdata/example.acme")
	if err != nil {
		t.Fatal(err)
	}
	m := check.NewModel(d)

	// The overlay's own reader has to declare PART-TYPE pins for any of this to work: the engine's
	// pin relations project from declared pins, not from net connections. A format that emits only
	// connections leaves them all empty and a pin rule silently finds nothing, so this guards the
	// reader half from regressing and taking the rule's evidence with it.
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
	for _, f := range r.Eval(m) {
		got = append(got, f.Subject)
	}
	// VCC carries U1's declared VDD power pin and X1. GND carries only ground-role pins, so the
	// rule must leave it alone — a pin-ROLE discrimination a net-level rule could not make, and
	// the reason this had to be a pin rule at all.
	if len(got) != 1 || got[0] != "VCC" {
		t.Errorf("datalog pin rule findings = %v, want exactly [VCC]", got)
	}
}
