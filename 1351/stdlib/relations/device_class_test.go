package relations

import (
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/param"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// efuseSpec is a seeded spec whose datasheet declares device_class "efuse". On a real industrial
// EVT export, no ref-des prefix or description keyword can establish that class (WS10-013). It has
// a source doc so the projected fact cites it. It has no parameters, since the class is a
// PartSpec-level field.
func efuseSpec(mpn string) *parampb.PartSpec {
	return &parampb.PartSpec{
		Mpn:          mpn,
		Manufacturer: "TI",
		DeviceClass:  "efuse",
		Docs:         []*parampb.SourceDoc{{Id: "ds", Title: mpn + " datasheet", Vendor: "TI"}},
	}
}

// TestComponentDeviceClassFact checks that a seeded, non-empty device_class projects a
// component.device_class row keyed by ref-des. The relation is empty when the model is built
// without a params tier, so it is silent by construction like the whole datasheet tier. core/check
// tests the check.Model side of WS10-013 (class-set enrichment, the Available gate), and this test
// covers the projector side.
func TestComponentDeviceClassFact(t *testing.T) {
	set := param.ParamSet{"TPS2HB16": efuseSpec("TPS2HB16")}
	m := check.NewModel(supplyDesign("+5V", false, "TPS2HB16"), check.WithParamProvider(set))
	rows := factsByRelation(Facts(m))[RelComponentDeviceClass]
	if len(rows) != 1 || rows[0].Subject != "U1" || rows[0].Value != "efuse" {
		t.Fatalf("component.device_class = %+v, want one (U1, efuse)", rows)
	}
	if len(rows[0].Cites) == 0 {
		t.Error("component.device_class row carries no Citation")
	}

	// No params tier attached -> the relation is empty (skip, never a false pass).
	bare := check.NewModel(supplyDesign("+5V", false, "TPS2HB16"))
	if rows := factsByRelation(Facts(bare))[RelComponentDeviceClass]; len(rows) != 0 {
		t.Errorf("component.device_class without --params = %+v, want empty", rows)
	}
}
