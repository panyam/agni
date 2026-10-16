package service

import (
	"testing"

	"github.com/panyam/agni/core/graph"
)

// TestTheConversionReportCarriesTheEngineLabel: the wire report names each component's summary group
// as graph.ComponentReport.Label decides it, so the viewer groups by the field rather than re-deriving
// the rule (agni issue 1045).
func TestTheConversionReportCarriesTheEngineLabel(t *testing.T) {
	r := &graph.ConversionReport{Components: []graph.ComponentReport{
		{RefDes: "R1", Kind: graph.KindGlyph, Class: "resistor"},
		{RefDes: "U1", Kind: "box", Class: "regulator"},
		{RefDes: "J1", Kind: "provided"},
	}}
	got := reportToProto(r).GetComponents()
	for i, c := range r.Components {
		if got[i].GetLabel() != c.Label() || got[i].GetLabel() == "" {
			t.Errorf("%s: label %q, want %q", c.RefDes, got[i].GetLabel(), c.Label())
		}
	}
	if got[0].GetLabel() != "resistor" || got[1].GetLabel() != "box" {
		t.Errorf("labels %q, %q; want resistor (a glyph's class) and box (its kind)", got[0].GetLabel(), got[1].GetLabel())
	}
}
