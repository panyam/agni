package check

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/datasheet/param"
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
	bare := NewModel(clockDesign("RES-1"))
	if !bare.HasClass("Y1", ClassClock) {
		t.Fatal("bare Y1 should be in the clock family")
	}
	if bare.HasClass("Y1", ClassCeramicResonator) {
		t.Error("bare Y1 must not be a ceramic_resonator without a datasheet")
	}
	if got := Run(bare, []*Rule{crystalLoadCaps}); len(got) != 2 {
		t.Fatalf("bare clock part with no load caps: want 2 findings (XIN, XOUT), got %d: %v", len(got), got)
	}

	// With the seeded "ceramic resonator": Y1 gains the ceramic_resonator subtype + the clock family tag,
	// and crystal-load-caps excludes it (integrated caps).
	seeded := NewModelWithParams(clockDesign("RES-1"), nil, param.ParamSet{"RES-1": resonatorSpec("RES-1")})
	if !seeded.HasClass("Y1", ClassCeramicResonator) {
		t.Error("seeded Y1 should carry the ceramic_resonator subtype (normalized from 'ceramic resonator')")
	}
	if !seeded.HasClass("Y1", ClassClock) {
		t.Error("seeded Y1 should still carry the clock family tag")
	}
	if got := Run(seeded, []*Rule{crystalLoadCaps}); len(got) != 0 {
		t.Errorf("a datasheet-declared ceramic resonator must be excluded from crystal-load-caps, got %v", got)
	}
}

// TestClockFamilyTagRetention: each clock subtype answers HasClass(clock) (family membership), and an
// oscillator does NOT answer HasClass(crystal) — the family is clock, not crystal (WS10-015).
func TestClockFamilyTagRetention(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "X1", DeviceClasses: []string{string(ClassOscillator), string(ClassClock)}},
		{RefDes: "Y1", DeviceClasses: []string{string(ClassCrystal), string(ClassClock)}},
		{RefDes: "Y2", DeviceClasses: []string{string(ClassCeramicResonator), string(ClassClock)}},
	}}
	m := NewModel(d)
	for _, ref := range []string{"X1", "Y1", "Y2"} {
		if !m.HasClass(ref, ClassClock) {
			t.Errorf("HasClass(%s, clock) should be true (family membership)", ref)
		}
	}
	if m.HasClass("X1", ClassCrystal) {
		t.Error("an oscillator must NOT satisfy HasClass(crystal) — the family is clock, not crystal")
	}
	if got := m.ComponentClass("X1"); got != ClassOscillator {
		t.Errorf("ComponentClass(X1) = %s, want oscillator (most-specific)", got)
	}
}
