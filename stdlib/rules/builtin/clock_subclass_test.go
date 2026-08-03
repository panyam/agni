package builtin

import (
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// clockDesign builds a two-terminal Y-prefix clock part (XIN/XOUT, no load caps) carrying an MPN, so
// crystal-load-caps fires on it unless a subtype excludes it.
func clockDesign(mpn string) *ir.Design {
	return &ir.Design{
		Components: []*ir.Component{{
			RefDes: "Y1", Attributes: map[string]string{"MPN": mpn},
			Prov: &ir.Provenance{SourceFile: "t"},
		}},
		Nets: []*ir.Net{
			{Name: "XIN", Connections: []*ir.Connection{{ComponentRef: "Y1", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "t"}},
			{Name: "XOUT", Connections: []*ir.Connection{{ComponentRef: "Y1", PinRef: "2"}}, Prov: &ir.Provenance{SourceFile: "t"}},
		},
	}
}

// resonatorSpec seeds a datasheet whose device_class is the spaced vendor synonym "ceramic resonator",
// exercising the normalization path (WS10-015).
func resonatorSpec(mpn string) *parampb.PartSpec {
	return &parampb.PartSpec{
		Mpn: mpn, Manufacturer: "Murata", DeviceClass: "ceramic resonator",
		Docs: []*parampb.SourceDoc{{Id: "ds", Title: mpn + " datasheet", Vendor: "Murata"}},
	}
}

// TestDatasheetSubtypesClockFamily: a bare Y clock part is the clock FAMILY (so crystal-load-caps acts
// on it), and a seeded device_class "ceramic resonator" enriches it to the ceramic_resonator subtype
// (via the normalization + family-tag path), which then EXCLUDES it from crystal-load-caps — the
// datasheet suppresses a false load-cap finding on an integrated-cap resonator.
func TestDatasheetSubtypesClockFamily(t *testing.T) {
	// Without a datasheet: Y1 is the ambiguous clock family, has no subtype, and fires (2 terminals, no caps).
	bare := check.NewModel(clockDesign("RES-1"))
	if !bare.HasClass("Y1", check.ClassClock) {
		t.Fatal("bare Y1 should be in the clock family")
	}
	if bare.HasClass("Y1", check.ClassCeramicResonator) {
		t.Error("bare Y1 must not be a ceramic_resonator without a datasheet")
	}
	if got := check.Run(bare, []*check.Rule{crystalLoadCaps}); len(got) != 2 {
		t.Fatalf("bare clock part with no load caps: want 2 findings (XIN, XOUT), got %d: %v", len(got), got)
	}

	// With the seeded "ceramic resonator": Y1 gains the ceramic_resonator subtype + the clock family tag,
	// and crystal-load-caps excludes it (integrated caps).
	seeded := check.NewModelWithParams(clockDesign("RES-1"), nil, param.ParamSet{"RES-1": resonatorSpec("RES-1")})
	if !seeded.HasClass("Y1", check.ClassCeramicResonator) {
		t.Error("seeded Y1 should carry the ceramic_resonator subtype (normalized from 'ceramic resonator')")
	}
	if !seeded.HasClass("Y1", check.ClassClock) {
		t.Error("seeded Y1 should still carry the clock family tag")
	}
	if got := check.Run(seeded, []*check.Rule{crystalLoadCaps}); len(got) != 0 {
		t.Errorf("a datasheet-declared ceramic resonator must be excluded from crystal-load-caps, got %v", got)
	}
}
