package relations

import (
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/datasheet/param"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// efuseSpec is a seeded spec whose datasheet declares device_class "efuse" (the class no ref-des
// prefix or description keyword on a real industrial EVT export can establish — WS10-013). It carries a
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
// construction, the whole datasheet tier's posture). The check.Model side of WS10-013 (class-set
// enrichment, the Available gate) is tested in core/check; this is the projector side.
func TestComponentDeviceClassFact(t *testing.T) {
	set := param.ParamSet{"TPS2HB16": efuseSpec("TPS2HB16")}
	m := check.NewModelWithParams(supplyDesign("+5V", false, "TPS2HB16"), nil, set)
	rows := factsByRelation(Facts(m))[RelComponentDeviceClass]
	if len(rows) != 1 || rows[0].Subject != "U1" || rows[0].Value != "efuse" {
		t.Fatalf("component.device_class = %+v, want one (U1, efuse)", rows)
	}
	if rows[0].Cite == "" {
		t.Error("component.device_class row carries no Citation")
	}

	// No params tier attached -> the relation is empty (skip, never a false pass).
	bare := check.NewModel(supplyDesign("+5V", false, "TPS2HB16"))
	if rows := factsByRelation(Facts(bare))[RelComponentDeviceClass]; len(rows) != 0 {
		t.Errorf("component.device_class without --params = %+v, want empty", rows)
	}
}
