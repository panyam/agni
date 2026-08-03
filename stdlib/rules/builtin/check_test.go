package builtin

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// tnet builds a net with "refdes.pin" connections.
func tnet(name string, conns ...string) *ir.Net {
	n := &ir.Net{Name: name, Prov: &ir.Provenance{SourceFile: "t"}}
	for _, c := range conns {
		p := strings.SplitN(c, ".", 2)
		n.Connections = append(n.Connections, &ir.Connection{ComponentRef: p[0], PinRef: p[1]})
	}
	return n
}

// ruleFixture is a labeled design exercising each rule (the seed of the rules-conformance
// suite): a real stub, a tool-generated unconnected net, a NO_CONNECT-pinned net, a normal
// net, an I2C net missing a pull-up, and an I2C net with one; plus an unconnected component
// and a section-connected one.
func ruleFixture() *ir.Design {
	return &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{
			{Name: "NCPART", Pins: []*ir.Pin{{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_NO_CONNECT}}},
		}}},
		Components: []*ir.Component{
			{RefDes: "R1", Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "R2", Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "R3", Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "U2", Sections: []*ir.ComponentSection{{PartRef: "NCPART", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "R9", Prov: &ir.Provenance{SourceFile: "t"}}, // on no net
		},
		Nets: []*ir.Net{
			tnet("STUB", "R1.1"),                  // real single-pin -> flag
			tnet("unconnected-(P5-Pad1)", "R2.1"), // tool no-connect marker -> skip
			tnet("NC", "U2.1"),                    // U2.1 is NO_CONNECT -> skip
			tnet("GND", "R1.2", "R2.2"),           // ok
			tnet("SDA", "U1.5"),                   // I2C, no pull-up -> flag
			tnet("SCL", "U1.6", "R3.1"),           // I2C, R3 pull-up -> ok
		},
	}
}

func indexFindings(d *ir.Design) map[string]check.Finding {
	m := map[string]check.Finding{}
	for _, f := range check.RunDesign(d) {
		m[f.Rule+"|"+f.Subject] = f
	}
	return m
}

func TestRules(t *testing.T) {
	f := indexFindings(ruleFixture())

	// single-pin-net: only the real stub; no-connect awareness skips the marker net and the
	// NO_CONNECT-pinned net.
	if _, ok := f["single-pin-net|STUB"]; !ok {
		t.Error("STUB should be flagged single-pin-net")
	}
	if _, ok := f["single-pin-net|unconnected-(P5-Pad1)"]; ok {
		t.Error("tool-generated unconnected-* net must be skipped (no-connect aware)")
	}
	if _, ok := f["single-pin-net|NC"]; ok {
		t.Error("net whose pin is NO_CONNECT must be skipped")
	}

	// unconnected-component: only R9; U2 is connected via its section pin (section-aware).
	if _, ok := f["unconnected-component|R9"]; !ok {
		t.Error("R9 should be flagged unconnected-component")
	}
	if _, ok := f["unconnected-component|U2"]; ok {
		t.Error("U2 is connected (section-aware); must not be flagged")
	}

	// i2c-pull-up: SDA missing a pull-up; SCL has R3.
	if _, ok := f["i2c-pull-up|SDA"]; !ok {
		t.Error("SDA should be flagged for missing pull-up")
	}
	if _, ok := f["i2c-pull-up|SCL"]; ok {
		t.Error("SCL has R3 pull-up; must not be flagged")
	}

	// Severities reviewed, and findings carry provenance.
	if got := f["single-pin-net|STUB"].Severity; got != "info" {
		t.Errorf("single-pin-net severity = %q, want info", got)
	}
	if got := f["unconnected-component|R9"].Severity; got != "warning" {
		t.Errorf("unconnected-component severity = %q, want warning", got)
	}
	if got := f["i2c-pull-up|SDA"].Severity; got != "error" {
		t.Errorf("i2c-pull-up severity = %q, want error", got)
	}
	if f["single-pin-net|STUB"].Prov == nil {
		t.Error("findings should carry provenance")
	}
}

// TestFindingKind checks that each finding carries the entity type of its subject, so a consumer
// can group/highlight by kind without re-guessing: net rules report KindNet, component rules
// KindComponent.
func TestFindingKind(t *testing.T) {
	f := indexFindings(ruleFixture())
	if got := f["single-pin-net|STUB"].Kind; got != check.KindNet {
		t.Errorf("single-pin-net STUB kind = %q, want %q", got, check.KindNet)
	}
	if got := f["i2c-pull-up|SDA"].Kind; got != check.KindNet {
		t.Errorf("i2c-pull-up SDA kind = %q, want %q", got, check.KindNet)
	}
	if got := f["unconnected-component|R9"].Kind; got != check.KindComponent {
		t.Errorf("unconnected-component R9 kind = %q, want %q", got, check.KindComponent)
	}
}

// tconn is one connection with an electrical direction, for the direction-rule fixtures.
type tconn struct {
	ref, pin string
	dir      ir.PinDirection
}

// dirDesign builds a design where each connection's pin carries a direction, so Model.PinDir
// resolves it: one part per ref_des, its pins accumulated from the connections. attrs sets each
// named net's attributes (e.g. power_driven, external).
func dirDesign(nets map[string][]tconn, attrs map[string]map[string]string) *ir.Design {
	d := &ir.Design{}
	lib := &ir.PartLibrary{Name: "lib"}
	part := map[string]*ir.PartType{}
	for nm, conns := range nets {
		net := &ir.Net{Name: nm, Prov: &ir.Provenance{SourceFile: "t"}, Attributes: attrs[nm]}
		for _, c := range conns {
			net.Connections = append(net.Connections, &ir.Connection{ComponentRef: c.ref, PinRef: c.pin})
			p := part[c.ref]
			if p == nil {
				p = &ir.PartType{Name: c.ref}
				part[c.ref] = p
				lib.Parts = append(lib.Parts, p)
				d.Components = append(d.Components, &ir.Component{RefDes: c.ref, Sections: []*ir.ComponentSection{{PartRef: c.ref, LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}})
			}
			p.Pins = append(p.Pins, &ir.Pin{Designator: c.pin, Direction: c.dir})
		}
		d.Nets = append(d.Nets, net)
	}
	d.Libraries = []*ir.PartLibrary{lib}
	return d
}

const (
	dIn    = ir.PinDirection_PIN_DIRECTION_INPUT
	dOut   = ir.PinDirection_PIN_DIRECTION_OUTPUT
	dPwrIn = ir.PinDirection_PIN_DIRECTION_POWER_IN
	dPas   = ir.PinDirection_PIN_DIRECTION_PASSIVE
)

// TestOutputOutputConflict: two hard drivers on a net fight (flag); one driver, or a bus of
// bidirectional pins, does not.
func TestOutputOutputConflict(t *testing.T) {
	f := indexFindings(dirDesign(map[string][]tconn{
		"FIGHT": {{"U1", "1", dOut}, {"U2", "1", dOut}},                                       // two outputs -> flag
		"OK":    {{"U3", "1", dOut}, {"U4", "1", dIn}},                                        // one driver -> fine
		"PWROK": {{"REG", "1", ir.PinDirection_PIN_DIRECTION_POWER_OUT}, {"U5", "1", dPwrIn}}, // one power source -> fine
	}, nil))
	if _, ok := f["output-output-conflict|FIGHT"]; !ok {
		t.Error("FIGHT should be flagged (two outputs)")
	}
	if _, ok := f["output-output-conflict|OK"]; ok {
		t.Error("OK has one driver; must not flag")
	}
	if _, ok := f["output-output-conflict|PWROK"]; ok {
		t.Error("PWROK has one power source; must not flag")
	}
	if got := f["output-output-conflict|FIGHT"].Severity; got != "error" {
		t.Errorf("severity = %q, want error", got)
	}
}

// TestFloatingInput: a net of only inputs floats (flag); an input with a driver or a pull, or a
// lone input (single-pin-net's job), does not.
func TestFloatingInput(t *testing.T) {
	f := indexFindings(dirDesign(map[string][]tconn{
		"FLOAT":  {{"U1", "1", dIn}, {"U2", "1", dIn}},  // two inputs, no driver -> flag
		"DRIVEN": {{"U3", "1", dIn}, {"U4", "1", dOut}}, // has a driver -> fine
		"PULLED": {{"U5", "1", dIn}, {"R1", "1", dPas}}, // has a pull -> fine
		"LONE":   {{"U6", "1", dIn}},                    // single pin -> single-pin-net, not this
	}, nil))
	if _, ok := f["floating-input|FLOAT"]; !ok {
		t.Error("FLOAT should be flagged (only inputs)")
	}
	for _, n := range []string{"DRIVEN", "PULLED", "LONE"} {
		if _, ok := f["floating-input|"+n]; ok {
			t.Errorf("%s must not be flagged floating-input", n)
		}
	}
}

// TestPowerInputNotDriven: a power-input with no source flags, unless a power source, a power flag,
// or a cross-sheet continuation could feed it.
func TestPowerInputNotDriven(t *testing.T) {
	f := indexFindings(dirDesign(map[string][]tconn{
		"DEAD":   {{"U1", "1", dPwrIn}, {"C1", "1", dPas}},                                     // power-in + passive, no source -> flag
		"FED":    {{"U2", "1", dPwrIn}, {"REG", "1", ir.PinDirection_PIN_DIRECTION_POWER_OUT}}, // power-out source -> fine
		"FLAG":   {{"U3", "1", dPwrIn}, {"C2", "1", dPas}},                                     // power_driven attr -> fine
		"XSHEET": {{"U4", "1", dPwrIn}, {"C3", "1", dPas}},                                     // external attr -> fine
	}, map[string]map[string]string{
		"FLAG":   {"power_driven": "true"},
		"XSHEET": {"external": "true"},
	}))
	if _, ok := f["power-input-not-driven|DEAD"]; !ok {
		t.Error("DEAD should be flagged (power-in, no source)")
	}
	for _, n := range []string{"FED", "FLAG", "XSHEET"} {
		if _, ok := f["power-input-not-driven|"+n]; ok {
			t.Errorf("%s must not be flagged power-input-not-driven", n)
		}
	}
}

// TestDuplicateRefDes checks the rule reports each ref-des collision the reader recorded, with the
// component as subject and the first colliding placement's provenance, and is silent when there are
// none.
func TestDuplicateRefDes(t *testing.T) {
	d := &ir.Design{InputDiagnostics: &ir.InputDiagnostics{RefDesCollisions: []*ir.RefDesCollision{
		{RefDes: "U1", Instances: []*ir.Provenance{{NativeId: "aaa"}, {NativeId: "bbb"}}},
	}}}
	f := indexFindings(d)
	got, ok := f["duplicate-ref-des|U1"]
	if !ok {
		t.Fatal("U1 collision should be flagged duplicate-ref-des")
	}
	if got.Kind != check.KindComponent {
		t.Errorf("kind = %q, want %q", got.Kind, check.KindComponent)
	}
	if got.Severity != "error" {
		t.Errorf("severity = %q, want error", got.Severity)
	}
	if got.Prov == nil || got.Prov.NativeId != "aaa" {
		t.Errorf("finding should carry the first colliding placement's provenance, got %+v", got.Prov)
	}
	if fs := check.RunDesign(&ir.Design{}); len(fs) != 0 {
		t.Errorf("design with no collisions produced %d finding(s), want 0", len(fs))
	}
}

// TestDanglingEndpoint checks that the rule reports each dangling endpoint the reader recorded,
// with the endpoint location as the subject and KindEndpoint, and stays silent when there are none.
func TestDanglingEndpoint(t *testing.T) {
	d := &ir.Design{InputDiagnostics: &ir.InputDiagnostics{DanglingEndpoints: []*ir.DanglingEndpoint{
		{X: 0, Y: 0, Prov: &ir.Provenance{SourceFile: "t", NativeId: "w1", NativeIdKind: "kicad-uuid"}},
		{X: 10, Y: -20, Prov: &ir.Provenance{SourceFile: "t"}},
	}}}
	f := indexFindings(d)
	got, ok := f["dangling-endpoint|0,0"]
	if !ok {
		t.Fatal("endpoint (0,0) should be flagged dangling-endpoint")
	}
	if got.Kind != check.KindEndpoint {
		t.Errorf("kind = %q, want %q", got.Kind, check.KindEndpoint)
	}
	if got.Severity != "warning" {
		t.Errorf("severity = %q, want warning", got.Severity)
	}
	if got.Prov == nil || got.Prov.NativeId != "w1" {
		t.Errorf("dangling finding should carry the wire's provenance, got %+v", got.Prov)
	}
	if _, ok := f["dangling-endpoint|10,-20"]; !ok {
		t.Error("endpoint (10,-20) should be flagged")
	}

	// No dangling endpoints -> rule silent.
	if fs := check.RunDesign(&ir.Design{}); len(fs) != 0 {
		t.Errorf("empty design produced %d finding(s), want 0", len(fs))
	}
}

// TestDiffPairNaming exercises the differential-pair rule: an orphaned positive member is
// flagged, a complete pair is not, and an active-low "_N" net is not mistaken for a diff half
// (positive-anchored detection). Nets carry two connections so single-pin-net stays quiet.
func TestDiffPairNaming(t *testing.T) {
	d := &ir.Design{Nets: []*ir.Net{
		tnet("HDMI_TX0_P", "U1.10", "J1.1"),  // orphaned positive -> flag (expects HDMI_TX0_N)
		tnet("HDMI_TX1_P", "U1.11", "J1.2"),  // paired
		tnet("HDMI_TX1_N", "U1.12", "J1.3"),  // paired
		tnet("USB_D+", "U1.20", "J1.4"),      // orphaned "+" -> flag (expects USB_D-)
		tnet("MIPI_CLK_DP", "U1.40", "J1.5"), // orphaned "_DP" -> flag (expects MIPI_CLK_DN)
		tnet("RESET_N", "U1.30", "U2.1"),     // active-low, NOT a diff half -> must not flag
	}}
	f := indexFindings(d)

	for _, sub := range []string{"HDMI_TX0_P", "USB_D+", "MIPI_CLK_DP"} {
		if _, ok := f["diff-pair-naming|"+sub]; !ok {
			t.Errorf("%s should be flagged diff-pair-naming (missing complement)", sub)
		}
	}
	for _, sub := range []string{"HDMI_TX1_P", "HDMI_TX1_N", "RESET_N"} {
		if _, ok := f["diff-pair-naming|"+sub]; ok {
			t.Errorf("%s must not be flagged diff-pair-naming", sub)
		}
	}
	if got := f["diff-pair-naming|HDMI_TX0_P"].Severity; got != "warning" {
		t.Errorf("diff-pair-naming severity = %q, want warning", got)
	}
	if f["diff-pair-naming|HDMI_TX0_P"].Prov == nil {
		t.Error("diff-pair findings should carry provenance")
	}
}

// TestDiffPairNamingNoConvention: a design where nothing is differential (only _P-suffixed
// nets, no _N siblings anywhere) must not fire. This is the LGSynth-benchmark profile that
// sprayed 190 warnings per file before the pair-population gate (WS3-024).
func TestDiffPairNamingNoConvention(t *testing.T) {
	d := &ir.Design{Nets: []*ir.Net{
		tnet("NA_P", "U1.1", "U2.1"),
		tnet("NB_P", "U1.2", "U2.2"),
		tnet("NAXZ0_P", "U1.3", "U2.3"),
	}}
	f := indexFindings(d)
	for k := range f {
		if strings.HasPrefix(k, "diff-pair-naming|") {
			t.Errorf("no diff-pair finding expected without a complete pair, got %q", k)
		}
	}
}
