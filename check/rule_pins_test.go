package check

import (
	"reflect"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// pinFixture: one two-section part sharing a power-pin designator across sections
// (dedup case), one part with a bare pin and an NC pin, and an NC pin wired into a
// real net.
func pinFixture() *ir.Design {
	return &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{
			{Name: "GATE_A", Pins: []*ir.Pin{
				{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_INPUT},
				{Designator: "14", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN}, // shared, listed per section
			}},
			{Name: "GATE_B", Pins: []*ir.Pin{
				{Designator: "2", Direction: ir.PinDirection_PIN_DIRECTION_INPUT},
				{Designator: "14", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN},
			}},
			{Name: "PART", Pins: []*ir.Pin{
				{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_INPUT},
				{Designator: "2", Direction: ir.PinDirection_PIN_DIRECTION_INPUT},       // typed + bare -> fires
				{Designator: "3", Direction: ir.PinDirection_PIN_DIRECTION_NO_CONNECT},  // bare NC -> silent
				{Designator: "4", Direction: ir.PinDirection_PIN_DIRECTION_NO_CONNECT},  // wired NC -> nc-pin-connected
				{Designator: "5", Direction: ir.PinDirection_PIN_DIRECTION_UNSPECIFIED}, // bare, direction unknown -> silent
			}},
		}}},
		Components: []*ir.Component{
			{RefDes: "U1", Sections: []*ir.ComponentSection{
				{PartRef: "GATE_A", LibraryRef: "lib"},
				{PartRef: "GATE_B", LibraryRef: "lib"},
			}, Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "U2", Sections: []*ir.ComponentSection{{PartRef: "PART", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "R1", Prov: &ir.Provenance{SourceFile: "t"}},
		},
		Nets: []*ir.Net{
			tnet("A", "U1.1", "U2.1"),
			tnet("VCC", "U1.14", "R1.1"),
			tnet("BADNC", "U2.4", "R1.2"),
			tnet("LONE_NC_STUB", "U2.3"), // single-member NC net: the intentional case, silent for both rules
		},
	}
}

func TestPinsModelSurface(t *testing.T) {
	m := NewModel(pinFixture())
	seen := map[string]int{}
	for _, p := range m.Pins() {
		seen[p.Component.RefDes+"."+p.Designator]++
	}
	if seen["U1.14"] != 1 {
		t.Errorf("shared section pin U1.14 enumerated %d times, want 1 (dedup by designator)", seen["U1.14"])
	}
	if len(seen) != 8 { // U1: 1, 2, 14 (dedup); U2: 1, 2, 3, 4, 5; R1 has no part type
		t.Errorf("enumerated %d distinct pins: %v", len(seen), seen)
	}
	if !m.PinConnected("U1", "14") || m.PinConnected("U2", "2") {
		t.Errorf("PinConnected: U1.14=%v (want true), U2.2=%v (want false)",
			m.PinConnected("U1", "14"), m.PinConnected("U2", "2"))
	}
}

// TestUnconnectedPin: fires per bare typed pin with the designator on the finding.
// U1.2 (unwired gate input) and U2.2 fire; U2.3 (bare NC), U2.4 (wired NC), and U2.5
// (bare but direction-unknown) stay silent.
func TestUnconnectedPin(t *testing.T) {
	m := NewModel(pinFixture())
	got := map[string]bool{}
	for _, f := range unconnectedPin.Eval(m) {
		if f.Kind != KindPin {
			t.Errorf("finding kind = %q, want %q", f.Kind, KindPin)
		}
		got[f.Subject+"."+f.Pin] = true
	}
	want := map[string]bool{"U1.2": true, "U2.2": true}
	if len(got) != len(want) {
		t.Fatalf("fired on %v, want %v", got, want)
	}
	for k := range want {
		if !got[k] {
			t.Errorf("expected firing on %s, got %v", k, got)
		}
	}
}

// TestNCPinConnected: the wired NC pin's net fires; the lone-stub NC case stays silent.
func TestNCPinConnected(t *testing.T) {
	m := NewModel(pinFixture())
	fs := ncPinConnected.Eval(m)
	if len(fs) != 1 || fs[0].Subject != "BADNC" || fs[0].Kind != KindNet {
		t.Fatalf("findings = %+v, want one KindNet finding on BADNC", fs)
	}
}

// TestOutputConflictCountsComponents (WS1-025 fallout): paralleled driving pins of ONE
// component are one driver (the real corpus: a driver IC with six output pads on one
// net); two different components' outputs still fire.
func TestOutputConflictCountsComponents(t *testing.T) {
	drv := &ir.PartType{Name: "DRV", Pins: []*ir.Pin{
		{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_OUTPUT},
		{Designator: "2", Direction: ir.PinDirection_PIN_DIRECTION_OUTPUT},
	}}
	comp := func(ref string) *ir.Component {
		return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"},
			Sections: []*ir.ComponentSection{{PartRef: "DRV", LibraryRef: "lib"}}}
	}
	d := &ir.Design{
		Libraries:  []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{drv}}},
		Components: []*ir.Component{comp("U1"), comp("U2")},
		Nets: []*ir.Net{
			tnet("PARALLEL", "U1.1", "U1.2"), // one component, two output pins -> silent
			tnet("FIGHT", "U1.1", "U2.1"),    // two components -> fires
		},
	}
	fs := outputOutputConflict.Eval(NewModel(d))
	if len(fs) != 1 || fs[0].Subject != "FIGHT" {
		t.Fatalf("findings = %+v, want exactly FIGHT", fs)
	}
}

// TestOutputConflictWiredOr (WS3-064): a multi-output net that carries a resistor is an intentional
// open-drain wired-OR bus (a shared interrupt/inhibit line, the resistor its pull), not contention,
// and stays silent; two push-pull outputs with no resistor still fight. The signal is the resistor's
// PRESENCE, name-independent, because a real pull runs to an auto-named rail or to ground.
func TestOutputConflictWiredOr(t *testing.T) {
	drv := &ir.PartType{Name: "DRV", Pins: []*ir.Pin{{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_OUTPUT}}}
	pwr := &ir.PartType{Name: "PWR", Pins: []*ir.Pin{{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_OUT}}}
	res := &ir.PartType{Name: "RES", Pins: []*ir.Pin{
		{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_UNSPECIFIED},
		{Designator: "2", Direction: ir.PinDirection_PIN_DIRECTION_UNSPECIFIED},
	}}
	comp := func(ref, part string) *ir.Component {
		return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"},
			Sections: []*ir.ComponentSection{{PartRef: part, LibraryRef: "lib"}}}
	}
	d := &ir.Design{
		Libraries:  []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{drv, pwr, res}}},
		Components: []*ir.Component{comp("U1", "DRV"), comp("U2", "DRV"), comp("U3", "DRV"), comp("U4", "DRV"),
			comp("PS1", "PWR"), comp("PS2", "PWR"), comp("R1", "RES"), comp("R2", "RES")},
		Nets: []*ir.Net{
			tnet("INT_B", "U1.1", "U2.1", "R1.1"),  // two signal outputs + a pull resistor -> wired-OR, silent
			tnet("FIGHT2", "U3.1", "U4.1"),         // two signal outputs, no resistor -> fires
			tnet("RAILFIGHT", "PS1.1", "PS2.1", "R2.1"), // two POWER sources + a resistor -> still a real conflict
		},
	}
	got := map[string]bool{}
	for _, f := range outputOutputConflict.Eval(NewModel(d)) {
		got[f.Subject] = true
	}
	if got["INT_B"] {
		t.Errorf("a wired-OR bus with a pull resistor must not fire output-output-conflict")
	}
	if !got["FIGHT2"] {
		t.Errorf("two push-pull outputs with no resistor must still fire")
	}
	if !got["RAILFIGHT"] {
		t.Errorf("two power sources on a net must still fire even with a resistor present")
	}
}

// TestUnspecifiedPinWithDriver: the matrix's unspecified column. An untyped pin fires
// only where a driver is in evidence — including a virtual power symbol's power_out
// (WS1-014), the evidence path that unblocked the row. Declared (passive) pins,
// undriven nets, external nets, and virtual pins as subjects all stay out.
func TestUnspecifiedPinWithDriver(t *testing.T) {
	lib := &ir.PartLibrary{Name: "lib", Parts: []*ir.PartType{
		{Name: "DRV", Pins: []*ir.Pin{{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_OUTPUT}}},
		{Name: "CONN", Pins: []*ir.Pin{
			{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_UNSPECIFIED},
			{Designator: "2", Direction: ir.PinDirection_PIN_DIRECTION_UNSPECIFIED},
			{Designator: "3", Direction: ir.PinDirection_PIN_DIRECTION_UNSPECIFIED},
		}},
		{Name: "RES", Pins: []*ir.Pin{{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_PASSIVE}}},
		{Name: "BUF", Pins: []*ir.Pin{{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_INPUT}}},
	}}
	comp := func(ref, part string) *ir.Component {
		return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"},
			Sections: []*ir.ComponentSection{{PartRef: part, LibraryRef: "lib"}}}
	}
	ext := tnet("EXT_DRIVEN", "U1.1", "J1.3")
	ext.Attributes = map[string]string{"external": "true"}
	d := &ir.Design{
		Libraries: []*ir.PartLibrary{lib},
		Components: []*ir.Component{comp("U1", "DRV"), comp("J1", "CONN"), comp("R1", "RES"), comp("U2", "BUF"),
			{RefDes: "X1", Prov: &ir.Provenance{SourceFile: "t"}}}, // no part type: pins undeclared
		Nets: []*ir.Net{
			tnet("DRIVEN_UNTYPED", "U1.1", "J1.1"),                        // output + declared unspecified -> fires
			vnet("RAIL", vconn("#FLG01", "1", "power_out"), conn("J1.2")), // virtual driver + declared unspecified -> fires
			tnet("DRIVEN_TYPED", "U1.1", "R1.1"),                          // output + passive (typed) -> silent
			tnet("UNDRIVEN", "J1.3", "U2.1"),                              // unspecified + input, no driver -> silent
			tnet("DRIVEN_UNKNOWN", "U1.1", "X1.7"),                        // driver + undeclared pin (read gap) -> silent
			ext,                                                           // driven + unspecified but cross-sheet -> silent
		},
	}
	got := map[string]bool{}
	for _, f := range unspecifiedPinWithDriver.Eval(NewModel(d)) {
		got[f.Subject] = true
	}
	want := map[string]bool{"DRIVEN_UNTYPED": true, "RAIL": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fired on %v, want %v", got, want)
	}
}

// TestFloatingInputPassiveExemption (WS1-025 fallout): a passive member silences the
// rule — a resistor is the classic pull, and libraries that type passive pins INPUT
// (the Mentor corpus's capacitors) must not turn cap+input nets into findings.
func TestFloatingInputPassiveExemption(t *testing.T) {
	inp := &ir.PartType{Name: "BUF", Pins: []*ir.Pin{
		{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_INPUT},
	}}
	capT := &ir.PartType{Name: "CAP", Pins: []*ir.Pin{ // pins typed INPUT, like the Mentor corpus
		{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_INPUT},
		{Designator: "2", Direction: ir.PinDirection_PIN_DIRECTION_INPUT},
	}}
	comp := func(ref, part string) *ir.Component {
		return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"},
			Sections: []*ir.ComponentSection{{PartRef: part, LibraryRef: "lib"}}}
	}
	d := &ir.Design{
		Libraries:  []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{inp, capT}}},
		Components: []*ir.Component{comp("U1", "BUF"), comp("U2", "BUF"), comp("C1", "CAP"), comp("U3", "BUF"), comp("U4", "BUF")},
		Nets: []*ir.Net{
			tnet("WITHCAP", "U1.1", "C1.1"), // input + INPUT-typed cap -> exempt, silent
			tnet("FLOATS", "U3.1", "U4.1"),  // two inputs, no passive -> fires
		},
	}
	m := NewModel(d)
	fs := floatingInput.Eval(m)
	if len(fs) != 1 || fs[0].Subject != "FLOATS" {
		t.Fatalf("findings = %+v, want exactly FLOATS", fs)
	}
}

// TestFloatingInputDiodeExemption: a diode/LED/TVS terminal is typed INPUT by some libraries
// but is not a logic input, so a pure diode network (e.g. two steering-diode cathodes tied
// together) must not read as floating; a real IC input that merely carries a clamp diode still
// does, since the exclusion is per-pin, not per-net.
func TestFloatingInputDiodeExemption(t *testing.T) {
	pin1 := func(name string) *ir.PartType {
		return &ir.PartType{Name: name, Pins: []*ir.Pin{
			{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_INPUT}, // typed INPUT, like the corpus
		}}
	}
	comp := func(ref, part string) *ir.Component {
		return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"},
			Sections: []*ir.ComponentSection{{PartRef: part, LibraryRef: "lib"}}}
	}
	d := &ir.Design{
		Libraries:  []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{pin1("BUF"), pin1("DIO")}}},
		Components: []*ir.Component{comp("D1", "DIO"), comp("D2", "DIO"), comp("U5", "BUF"), comp("D3", "DIO")},
		Nets: []*ir.Net{
			tnet("DIODENET", "D1.1", "D2.1"), // two diode terminals -> not a logic input, silent
			tnet("CLAMPED", "U5.1", "D3.1"),  // real IC input + a clamp diode -> still fires
		},
	}
	got := map[string]bool{}
	for _, f := range floatingInput.Eval(NewModel(d)) {
		got[f.Subject] = true
	}
	if got["DIODENET"] {
		t.Errorf("a pure diode network must not fire floating-input")
	}
	if !got["CLAMPED"] {
		t.Errorf("a real IC input carrying a clamp diode must still fire")
	}
	if len(got) != 1 {
		t.Errorf("want exactly CLAMPED, got %v", got)
	}
}

// TestTestPointCoverage: uncovered rails and ground fire ONLY on boards that place test
// points at all (the DFT channel gate); covered rails, external rails, and plain signal
// nets stay out.
func TestTestPointCoverage(t *testing.T) {
	comp := func(ref string) *ir.Component {
		return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"}}
	}
	ext := tnet("VBUS", "U1.3")
	ext.Attributes = map[string]string{"external": "true"}
	withTP := &ir.Design{
		Components: []*ir.Component{comp("TP1"), comp("TP2"), comp("U1"), comp("R1")},
		Nets: []*ir.Net{
			tnet("VCC", "U1.1", "TP1.1"),    // covered rail -> silent
			tnet("3V3", "U1.2", "R1.1"),     // rail name, no TP -> fires
			tnet("GND", "U1.4", "TP2.1"),    // covered ground -> silent
			tnet("SIG", "U1.5", "R1.2"),     // not a rail -> silent
			tnet("VCC1V2_FB", "U1.6", "R1.3"), // rail-named feedback sense node -> not probeable, silent (WS3-067)
			ext,                             // rail name but cross-sheet -> silent
		},
	}
	got := map[string]bool{}
	for _, f := range testPointCoverage.Eval(NewModel(withTP)) {
		got[f.Subject] = true
	}
	if len(got) != 1 || !got["3V3"] {
		t.Errorf("fired on %v, want exactly 3V3", got)
	}

	noTP := &ir.Design{
		Components: []*ir.Component{comp("U1"), comp("R1")},
		Nets:       []*ir.Net{tnet("VCC", "U1.1", "R1.1"), tnet("GND", "U1.2", "R1.2")},
	}
	if fs := testPointCoverage.Eval(NewModel(noTP)); len(fs) != 0 {
		t.Errorf("zero-TP board has no DFT convention to violate; fired %+v", fs)
	}
}
