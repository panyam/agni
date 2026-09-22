package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestRenderReadsTheProjectsDatasheetClasses is the acceptance for agni issue 710, at the surface a
// user drives. The design under test states three parts a netlist cannot type: a bare crystal, a
// ceramic resonator and an ESD clamp, each identified only by the device_class its datasheet states.
//
// It goes through the CLI rather than the pass because the whole defect was a wiring one. Every piece
// existed: the corpus was discovered, the classes were derived, the query answered with them. They
// reached check.Model and stopped there, so the drawing re-derived its own answer from the ref-des
// prefix and disagreed. What this asserts is that pointing the renderer at a design inside a project
// is enough, with no flag, which is also the property that makes the viewer agree with the query
// panel beside it.
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
	// The positive control. Both classes below are what the CONVENTION tier answers for these parts,
	// so a report still carrying them is one where the datasheet tier never fired, and the assertions
	// above would be measuring a fixture rather than a behaviour.
	for _, stale := range []string{"  clock ", "  diode "} {
		if strings.Contains(out, stale) {
			t.Errorf("report still groups by the keyword class %q:\n%s", strings.TrimSpace(stale), out)
		}
	}
}
