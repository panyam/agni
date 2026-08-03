package check

import (
	"testing"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/datasheet/param"
)

// efuseSpec is a seeded spec whose datasheet declares device_class "efuse" (the class no ref-des
// prefix or description keyword on a real automotive EVT export can establish — WS10-013). It carries a
// source doc so the projected fact cites it, and no parameters (the class is a PartSpec-level field).
func efuseSpec(mpn string) *parampb.PartSpec {
	return &parampb.PartSpec{
		Mpn:          mpn,
		Manufacturer: "TI",
		DeviceClass:  "efuse",
		Docs:         []*parampb.SourceDoc{{Id: "ds", Title: mpn + " datasheet", Vendor: "TI"}},
	}
}

// TestComponentDeviceClassFact: a seeded, non-empty device_class projects a component.device_class row
// keyed by ref-des, and the relation is empty when the model is built without a params tier (silent by
// construction, the whole datasheet tier's posture).
func TestComponentDeviceClassFact(t *testing.T) {
	set := param.ParamSet{"TPS2HB16": efuseSpec("TPS2HB16")}
	m := NewModelWithParams(supplyDesign("+5V", false, "TPS2HB16"), nil, set)
	rows := factsByRelation(Facts(m))[RelComponentDeviceClass]
	if len(rows) != 1 || rows[0].Subject != "U1" || rows[0].Value != "efuse" {
		t.Fatalf("component.device_class = %+v, want one (U1, efuse)", rows)
	}
	if rows[0].Cite == "" {
		t.Error("component.device_class row carries no citation")
	}

	// No params tier attached -> the relation is empty (skip, never a false pass).
	bare := NewModel(supplyDesign("+5V", false, "TPS2HB16"))
	if rows := factsByRelation(Facts(bare))[RelComponentDeviceClass]; len(rows) != 0 {
		t.Errorf("component.device_class without --params = %+v, want empty", rows)
	}
}

// TestDeviceClassEnrichesClassSet (WS10-013 Phase 2): a seeded device_class is merged into the
// component's device_classes SET, so HasClass answers from the datasheet — but only when a params tier
// is attached, and the keyword-derived most-specific class is left unchanged (additive, not promoted).
func TestDeviceClassEnrichesClassSet(t *testing.T) {
	set := param.ParamSet{"TPS2HB16": efuseSpec("TPS2HB16")}
	m := NewModelWithParams(supplyDesign("+5V", false, "TPS2HB16"), nil, set)

	// U1 keyword-classifies as ic (U prefix); the datasheet adds efuse as a membership tag.
	if !m.HasClass("U1", "efuse") {
		t.Error("HasClass(U1, efuse) should be true once the datasheet class is merged")
	}
	if !m.HasClass("U1", ClassIC) {
		t.Error("HasClass(U1, ic) should still be true (enrichment is additive)")
	}
	if got := m.ComponentClass("U1"); got != ClassIC {
		t.Errorf("ComponentClass(U1) = %s, want ic (a datasheet class is not promoted to most-specific)", got)
	}

	// Without a params tier the keyword class stands alone (degrade-safe, C9).
	bare := NewModel(supplyDesign("+5V", false, "TPS2HB16"))
	if bare.HasClass("U1", "efuse") {
		t.Error("HasClass(U1, efuse) should be false without --params (no datasheet to enrich from)")
	}
}

// TestDeviceClassRelationAvailability: a rule reading component.device_class is not-applicable without
// a seeded params set (so a review item bound to it reads not-automated, not a hollow pass), and
// applicable once a params tier is attached — the same gate the param(...) reads get.
func TestDeviceClassRelationAvailability(t *testing.T) {
	r := &Rule{Reads: []string{RelComponentDeviceClass}}
	if ok, reason := Available(r, NewModel(supplyDesign("+5V", false, "TPS2HB16"))); ok || reason == "" {
		t.Errorf("component.device_class rule on a params-less model: got (%v, %q), want (false, non-empty)", ok, reason)
	}
	seeded := NewModelWithParams(supplyDesign("+5V", false, "TPS2HB16"), nil, param.ParamSet{})
	if ok, _ := Available(r, seeded); !ok {
		t.Error("component.device_class rule with a params tier attached: want available")
	}
}

// TestEsdRatedRelationAvailability (WS3-087): component.esd_rated is a datasheet-tier relation whose
// name is not param-prefixed, so it must gate to not-applicable without --params exactly like
// component.device_class — else a datalog rule reading it silently passes on an unseeded design.
func TestEsdRatedRelationAvailability(t *testing.T) {
	r := &Rule{Reads: []string{RelEsdRated}}
	if ok, reason := Available(r, NewModel(supplyDesign("+5V", false, "TPS2HB16"))); ok || reason == "" {
		t.Errorf("component.esd_rated rule on a params-less model: got (%v, %q), want (false, non-empty)", ok, reason)
	}
	seeded := NewModelWithParams(supplyDesign("+5V", false, "TPS2HB16"), nil, param.ParamSet{})
	if ok, _ := Available(r, seeded); !ok {
		t.Error("component.esd_rated rule with a params tier attached: want available")
	}
}
