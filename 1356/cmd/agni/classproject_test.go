package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestRenderReadsTheProjectsDatasheetClasses is the acceptance for agni issue 710, at the surface a
// user drives. The design under test holds a bare crystal, a ceramic resonator and an ESD clamp. A
// netlist cannot type any of the three. Each is identified only by the device_class stated in its
// datasheet.
//
// It goes through the CLI rather than the pass because the whole defect was a wiring one. Every piece
// existed. The corpus was discovered, the classes were derived, and the query answered with them. They
// reached check.Model and stopped there, so the drawing re-derived its own answer from the ref-des
// prefix and disagreed. This test asserts that pointing the renderer at a design inside a project is
// enough, with no flag. The same property makes the viewer agree with the query panel beside it.
func TestRenderReadsTheProjectsDatasheetClasses(t *testing.T) {
	const design = "testdata/classproject/designs/clocks"
	var b bytes.Buffer
	if err := writeReport(&b, design+"/clocks.edn", symbolsGlyph, nil, "text"); err != nil {
		t.Fatalf("writeReport: %v", err)
	}
	out := b.String()
	for _, want := range []string{"crystal", "ceramic_resonator", "tvs"} {
		if !strings.Contains(out, want) {
			t.Errorf("the conversion report never says %q, so the drawing did not read the project's corpus:\n%s", want, out)
		}
	}
	// The positive control. The CONVENTION tier answers both classes below for these parts. If the
	// report still carries them, the datasheet tier never fired, and the assertions above would be
	// measuring a fixture rather than a behaviour.
	for _, stale := range []string{"  clock ", "  diode "} {
		if strings.Contains(out, stale) {
			t.Errorf("report still groups by the keyword class %q:\n%s", strings.TrimSpace(stale), out)
		}
	}
}
