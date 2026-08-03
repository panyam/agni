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
