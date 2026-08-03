package check

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/param"
)

// protDesign builds a design whose parts give pins electrical directions and whose nets carry
// the given attribute maps, for the rail/protection rule guards.
func protDesign(comps []*ir.Component, lib []*ir.PartType, nets []*ir.Net) *ir.Design {
	return &ir.Design{
		Libraries:  []*ir.PartLibrary{{Name: "lib", Parts: lib}},
		Components: comps,
		Nets:       nets,
	}
}

func firedSubjects(d *ir.Design, rule string) map[string]bool {
	out := map[string]bool{}
	for _, f := range RunDesign(d) {
		if f.Rule == rule {
			out[f.Subject] = true
		}
	}
	return out
}

func withAttrs(n *ir.Net, kv ...string) *ir.Net {
	n.Attributes = map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		n.Attributes[kv[i]] = kv[i+1]
	}
	return n
}

// TestBulkCap: a named rail (global or power_driven) with no capacitor fires; a rail with a
// cap, a ground-named rail, an unresolved external rail, and an unnamed net stay quiet.
func TestBulkCap(t *testing.T) {
	comps := []*ir.Component{
		{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "t"}},
		{RefDes: "C1", Prov: &ir.Provenance{SourceFile: "t"}},
	}
	d := protDesign(comps, nil, []*ir.Net{
		withAttrs(tnet("5V", "U1.1"), "global", "true"),          // named rail, no cap -> fires
		withAttrs(tnet("3V3", "U1.2", "C1.1"), "global", "true"), // has cap -> quiet
		withAttrs(tnet("GND", "U1.3"), "global", "true"),         // ground-named -> quiet
		withAttrs(tnet("VIN", "U1.4"), "external", "true"),       // unresolved external -> quiet
		withAttrs(tnet("VDRV", "U1.5"), "power_driven", "true"),  // flagged rail, no cap -> fires
		tnet("DATA", "U1.6", "C1.2"),                             // unnamed net -> not selected
	})
	got := firedSubjects(d, "bulk-cap")
	for _, want := range []string{"5V", "VDRV"} {
		if !got[want] {
			t.Errorf("%s should be flagged by bulk-cap", want)
		}
	}
	for _, quiet := range []string{"3V3", "GND", "VIN", "DATA"} {
		if got[quiet] {
			t.Errorf("%s must not be flagged by bulk-cap", quiet)
		}
	}
}

// TestInputProtection: a net where a connector directly feeds a power-input pin must carry a
// fuse or TVS; a series fuse splits the net so protected boards are not even selected.
func TestInputProtection(t *testing.T) {
	lib := []*ir.PartType{
		{Name: "REG", Pins: []*ir.Pin{{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN}}},
	}
	sec := []*ir.ComponentSection{{PartRef: "REG", LibraryRef: "lib"}}
	comps := []*ir.Component{
		{RefDes: "J1", Prov: &ir.Provenance{SourceFile: "t"}},
		{RefDes: "J2", Prov: &ir.Provenance{SourceFile: "t"}},
		{RefDes: "U1", Sections: sec, Prov: &ir.Provenance{SourceFile: "t"}},
		{RefDes: "U2", Sections: sec, Prov: &ir.Provenance{SourceFile: "t"}},
		{RefDes: "U3", Sections: sec, Prov: &ir.Provenance{SourceFile: "t"}},
		{RefDes: "F1", Prov: &ir.Provenance{SourceFile: "t"}},
		{RefDes: "D1", Prov: &ir.Provenance{SourceFile: "t", NativeId: "tvs"}},
	}
	comps[6].Attributes = map[string]string{"Value": "TVS"}
	comps = append(comps,
		&ir.Component{RefDes: "FB1", Prov: &ir.Provenance{SourceFile: "t"}},
		&ir.Component{RefDes: "FB2", Prov: &ir.Provenance{SourceFile: "t"}},
		&ir.Component{RefDes: "F2", Prov: &ir.Provenance{SourceFile: "t"}},
		&ir.Component{RefDes: "J3", Prov: &ir.Provenance{SourceFile: "t"}},
		&ir.Component{RefDes: "J4", Prov: &ir.Provenance{SourceFile: "t"}},
		&ir.Component{RefDes: "U4", Sections: sec, Prov: &ir.Provenance{SourceFile: "t"}},
		&ir.Component{RefDes: "U5", Sections: sec, Prov: &ir.Provenance{SourceFile: "t"}},
	)
	d := protDesign(comps, lib, []*ir.Net{
		tnet("VIN_BAD", "J1.1", "U1.1"),           // connector -> power_in, nothing else -> fires
		tnet("VIN_FUSED", "J2.1", "F1.1", "U2.1"), // fuse member -> quiet
		tnet("VIN_TVS", "J2.2", "D1.1", "U3.1"),   // tvs member -> quiet
		tnet("GND", "J1.4", "U1.3"),               // ground-named -> quiet
		tnet("SIG", "J1.2", "U1.9"),               // no power_in -> not selected
		// Reach cases (WS3-011): the power input sits BEHIND a series element.
		tnet("VIN_BEAD", "J3.1", "FB1.1"), // connector -> bead ...
		tnet("VIN_POST", "FB1.2", "U4.1"), // ... -> power_in, no protector anywhere -> fires
		tnet("VIN_F", "J4.1", "F2.1"),     // connector -> fuse ...
		tnet("VIN_F2", "F2.2", "FB2.1"),   // ... -> bead ...
		tnet("VIN_F3", "FB2.2", "U5.1"),   // ... -> power_in, fuse crossed -> quiet
	})
	got := firedSubjects(d, "input-protection")
	if !got["VIN_BAD"] {
		t.Error("VIN_BAD (connector feeding power_in, unprotected) should be flagged")
	}
	if !got["VIN_BEAD"] {
		t.Error("VIN_BEAD (power input one bead away, unprotected) should be flagged (WS3-011)")
	}
	for _, quiet := range []string{"VIN_FUSED", "VIN_TVS", "GND", "SIG", "VIN_F", "VIN_F2", "VIN_F3", "VIN_POST"} {
		if got[quiet] {
			t.Errorf("%s must not be flagged by input-protection", quiet)
		}
	}
}

// TestEsdProtection: an external signal net (connector member, no power pins, not a rail)
// needs a TVS member; protected, rail, ground, and connector-free nets stay quiet.
func TestEsdProtection(t *testing.T) {
	lib := []*ir.PartType{
		{Name: "REG", Pins: []*ir.Pin{{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN}}},
	}
	comps := []*ir.Component{
		{RefDes: "J1", Prov: &ir.Provenance{SourceFile: "t"}},
		{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "t"}},
		{RefDes: "U2", Sections: []*ir.ComponentSection{{PartRef: "REG", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}},
		{RefDes: "D1", Attributes: map[string]string{"Value": "TVS"}, Prov: &ir.Provenance{SourceFile: "t"}},
	}
	comps = append(comps, &ir.Component{RefDes: "R9", Prov: &ir.Provenance{SourceFile: "t"}})
	// A debug connector (WS3-066): a bench interface, not harness exposure, so its lines are not flagged.
	comps = append(comps, &ir.Component{RefDes: "J99", Attributes: map[string]string{"Description": "Debugger"}, Prov: &ir.Provenance{SourceFile: "t"}})
	d := protDesign(comps, lib, []*ir.Net{
		tnet("DP", "J1.2", "U1.5"),         // external signal, no tvs -> fires
		tnet("DBGSIG", "J99.1", "U1.12"),   // debug connector, no tvs -> NOT external, quiet
		tnet("DM", "J1.3", "U1.6", "D1.1"), // tvs member -> quiet
		tnet("VIN", "J1.1", "U2.1"),        // feeds a power pin -> input-protection's turf, quiet here
		tnet("GND", "J1.4", "U1.2"),        // ground-named -> quiet
		tnet("INT", "U1.7", "U1.8"),        // no connector -> not external
		tnet("VCC", "J1.5", "U1.9"),        // rail-named (directionless source) -> quiet
		tnet("12V", "J1.6", "U1.10"),       // digit-V rail name -> quiet
		// Reach case (WS3-011): the clamp sits one series hop behind the connector.
		tnet("DSER", "J1.7", "R9.1"),            // connector -> series R ...
		tnet("DCLAMP", "R9.2", "U1.11", "D1.2"), // ... -> clamped node -> DSER quiet
	})
	got := firedSubjects(d, "esd-protection")
	if !got["DP"] {
		t.Error("DP (external signal, no TVS) should be flagged")
	}
	for _, quiet := range []string{"DM", "VIN", "GND", "INT", "VCC", "12V", "DSER", "DCLAMP", "DBGSIG"} {
		if got[quiet] {
			t.Errorf("%s must not be flagged by esd-protection", quiet)
		}
	}
}

// TestEsdClampNotTVS (WS3-078): esd-protection and esd-clamp-not-tvs partition the unprotected
// external-signal nets — a bare net is esd-protection, a Zener-clamped net (no TVS) is
// esd-clamp-not-tvs, a TVS-clamped net is neither, and the two rules never both fire on one net.
func TestEsdClampNotTVS(t *testing.T) {
	comps := []*ir.Component{
		{RefDes: "J1", Prov: &ir.Provenance{SourceFile: "t"}},
		{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "t"}},
		{RefDes: "D1", Attributes: map[string]string{"Description": "DIODE ZENER 18V 1W"}, Prov: &ir.Provenance{SourceFile: "t"}},
		{RefDes: "D2", Attributes: map[string]string{"Value": "TVS"}, Prov: &ir.Provenance{SourceFile: "t"}},
	}
	d := protDesign(comps, nil, []*ir.Net{
		tnet("ZCLAMP", "J1.1", "U1.1", "D1.1"), // zener clamp, no TVS -> esd-clamp-not-tvs
		tnet("BARE", "J1.2", "U1.2"),           // nothing -> esd-protection
		tnet("TCLAMP", "J1.3", "U1.3", "D2.1"), // TVS -> neither
	})
	clamp := firedSubjects(d, "esd-clamp-not-tvs")
	prot := firedSubjects(d, "esd-protection")
	if !clamp["ZCLAMP"] {
		t.Error("ZCLAMP (zener clamp, no TVS) should be flagged by esd-clamp-not-tvs")
	}
	if !prot["BARE"] {
		t.Error("BARE (no protection) should be flagged by esd-protection")
	}
	for _, q := range []string{"BARE", "TCLAMP"} {
		if clamp[q] {
			t.Errorf("%s must not be flagged by esd-clamp-not-tvs", q)
		}
	}
	for _, q := range []string{"ZCLAMP", "TCLAMP"} {
		if prot[q] {
			t.Errorf("%s must not be flagged by esd-protection (moved to esd-clamp-not-tvs or protected)", q)
		}
	}
}

// esdSpec builds a synthetic transceiver spec declaring an ESD tolerance (unconditional, so it is
// machine-comparable — the test model lives in the name), at `volts`.
func esdSpec(mpn string, volts float64) *parampb.PartSpec {
	f := func(v float64) *float64 { return &v }
	return &parampb.PartSpec{
		Mpn: mpn, Manufacturer: "Agni",
		Docs: []*parampb.SourceDoc{{Id: "ds", Title: "DEMO-XCVR Rev A", Vendor: "Agni"}},
		Parameters: []*parampb.Parameter{{
			Name:              "ESD (IEC 61000-4-2, bus pins)",
			Symbol:            "V_ESD",
			LimitKind:         parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX,
			Value:             &parampb.RangeValue{Max: f(volts)},
			Unit:              "V",
			ConditionCoverage: parampb.ConditionCoverage_CONDITION_COVERAGE_UNCONDITIONAL,
			Attributes:        map[string]string{"esd_test_model": "iec"}, // system-level (WS3-077)
			Prov:              &parampb.ParamProvenance{DocRef: "ds", Page: 1, TableOrFigure: "ESD Ratings", Method: "hand", Confidence: 1},
		}},
	}
}

// TestEsdTestModelGate (WS3-077): the IC-ESD credit honors only a SYSTEM-level (IEC) rating. A handling
// model (HBM) at the same voltage does NOT credit, and an unstated test model does not either, so a
// harness input reaching an IC with only a handling rating stays flagged.
func TestEsdTestModelGate(t *testing.T) {
	comps := []*ir.Component{
		{RefDes: "J1", Prov: &ir.Provenance{SourceFile: "t"}},
		{RefDes: "U9", Attributes: map[string]string{"MPN": "DEMO-HBM"}, Prov: &ir.Provenance{SourceFile: "t"}},
	}
	d := protDesign(comps, nil, []*ir.Net{tnet("SIG", "J1.1", "U9.1")})
	// Same 8 kV, but the test model is HBM (handling), not IEC (system) -> must not credit.
	hbm := esdSpec("DEMO-HBM", 8000)
	hbm.Parameters[0].Attributes["esd_test_model"] = "hbm"
	fired := map[string]bool{}
	for _, fnd := range esdProtection.Eval(NewModelWithParams(d, nil, param.ParamSet{"DEMO-HBM": hbm})) {
		fired[fnd.Subject] = true
	}
	if !fired["SIG"] {
		t.Error("an HBM (handling) ESD rating must NOT credit a harness-exposed signal; SIG should still fire")
	}
}

// TestEsdIcRating (WS3-073): a connector-facing signal reaching an IC whose datasheet declares an ESD
// rating at or above the credit floor is protected (no finding, IC-integrated ESD); a signal to an
// unrated part still fires, and a rating below the floor does not credit.
func TestEsdIcRating(t *testing.T) {
	comps := []*ir.Component{
		{RefDes: "J1", Prov: &ir.Provenance{SourceFile: "t"}},
		{RefDes: "U9", Attributes: map[string]string{"MPN": "DEMO-XCVR"}, Prov: &ir.Provenance{SourceFile: "t"}},
		{RefDes: "U8", Prov: &ir.Provenance{SourceFile: "t"}},
	}
	d := protDesign(comps, nil, []*ir.Net{
		tnet("SIG_RATED", "J1.1", "U9.1"), // connector -> transceiver with an ESD rating -> credited, silent
		tnet("SIG_BARE", "J1.2", "U8.1"),  // connector -> unrated IC -> fires
	})
	fired := func(set param.ParamSet) map[string]bool {
		got := map[string]bool{}
		for _, f := range esdProtection.Eval(NewModelWithParams(d, nil, set)) {
			got[f.Subject] = true
		}
		return got
	}
	rated := fired(param.ParamSet{"DEMO-XCVR": esdSpec("DEMO-XCVR", 8000)}) // 8kV, above the 2kV floor
	if rated["SIG_RATED"] {
		t.Error("a connector signal to an IC with an ESD rating >= floor must not fire")
	}
	if !rated["SIG_BARE"] {
		t.Error("a connector signal to an unrated IC must still fire")
	}
	if !fired(param.ParamSet{"DEMO-XCVR": esdSpec("DEMO-XCVR", 500)})["SIG_RATED"] {
		t.Error("an ESD rating below the credit floor must not protect the signal")
	}
}

// TestEsdAvailableWithoutParams (WS3-073): esd-protection's param read is an optional exemption,
// so the rule stays applicable (and reports) on a design with no seeded set — the review runner
// must not mark it not-applicable. A genuine datasheet rule (supply-exceeds-abs-max) still gates.
func TestEsdAvailableWithoutParams(t *testing.T) {
	m := NewModel(protDesign([]*ir.Component{{RefDes: "J1", Prov: &ir.Provenance{SourceFile: "t"}}}, nil, nil))
	if ok, reason := Available(esdProtection, m); !ok {
		t.Errorf("esd-protection must be available without --params (optional param read), got not-applicable: %q", reason)
	}
	if ok, _ := Available(supplyExceedsAbsMax, m); ok {
		t.Error("supply-exceeds-abs-max must remain not-applicable without --params (required datasheet read)")
	}
}
