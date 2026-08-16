package geda

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func testOpener(t *testing.T) SymbolOpener {
	t.Helper()
	return func(symref string) ([]byte, error) {
		return os.ReadFile(filepath.Join("testdata", filepath.Base(symref)))
	}
}

func TestIsGeda(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"v 20200319 2\n", true},
		{"v 20031231 1\n", true},
		{"v {xschem version=3.4.4}\n", false}, // xschem
		{"(kicad_sch", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsGeda([]byte(c.in)); got != c.want {
			t.Errorf("IsGeda(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestReadStructural(t *testing.T) {
	d, err := Read(bytes.NewReader(readFixture(t, "divider.sch")), "divider.sch")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if d.SourceFormat != "geda" {
		t.Errorf("source format = %q, want geda", d.SourceFormat)
	}
	if len(d.Components) != 2 || d.Components[0].RefDes != "R1" || d.Components[1].RefDes != "R2" {
		t.Fatalf("components = %v, want R1,R2", refs(d))
	}
	if got := d.Components[1].Sections[0].Attributes["value"]; got != "2k" {
		t.Errorf("R2 value = %q, want 2k", got)
	}
	// Structural (no symbols): net names from netname= are present, but no connections.
	if got := netNames(d); !equalSet(got, []string{"IN", "MID"}) {
		t.Errorf("net names = %v, want IN,MID", got)
	}
}

func TestReadWithSymbols(t *testing.T) {
	d, err := ReadWithSymbols(bytes.NewReader(readFixture(t, "divider.sch")), "divider.sch", testOpener(t))
	if err != nil {
		t.Fatalf("ReadWithSymbols: %v", err)
	}
	want := map[string][]string{
		"IN":  {"R2.1"},
		"MID": {"R1.1", "R2.2"},
		"GND": {"R1.2"},
	}
	for name, conns := range want {
		if got := connsOf(d, name); !equalSet(got, conns) {
			t.Errorf("net %q = %v, want %v", name, got, conns)
		}
	}
}

func refs(d *ir.Design) []string {
	var out []string
	for _, c := range d.Components {
		out = append(out, c.RefDes)
	}
	return out
}

func netNames(d *ir.Design) []string {
	var out []string
	for _, n := range d.Nets {
		out = append(out, n.Name)
	}
	return out
}

func connsOf(d *ir.Design, net string) []string {
	var out []string
	for _, n := range d.Nets {
		if n.Name != net {
			continue
		}
		for _, c := range n.Connections {
			out = append(out, c.ComponentRef+"."+c.PinRef)
		}
	}
	return out
}

func equalSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := map[string]int{}
	for _, s := range a {
		m[s]++
	}
	for _, s := range b {
		m[s]--
	}
	for _, v := range m {
		if v != 0 {
			return false
		}
	}
	return true
}

// TestDanglingEndpoints (WS1-013): a wire drawn to nothing surfaces as a dangling
// endpoint. gEDA's netgraph grid is native (round only), so it IS the geometry frame —
// no unquant, unlike xschem. Emission is gated on full symbol resolution.
func TestDanglingEndpoints(t *testing.T) {
	d, err := ReadWithSymbols(bytes.NewReader(readFixture(t, "dangle.sch")), "dangle.sch", testOpener(t))
	if err != nil {
		t.Fatal(err)
	}
	dangles := d.GetInputDiagnostics().GetDanglingEndpoints()
	if len(dangles) != 1 {
		t.Fatalf("dangles = %d, want 1: %+v", len(dangles), dangles)
	}
	if dangles[0].X != 1000 || dangles[0].Y != 2400 {
		t.Errorf("dangle at (%d,%d), want (1000,2400)", dangles[0].X, dangles[0].Y)
	}
	if dangles[0].Prov.GetNativeId() != "" {
		t.Errorf("gEDA wires carry no id; got %q", dangles[0].Prov.GetNativeId())
	}
}

// TestDanglingSuppressedOnUnresolved: an opener that cannot resolve resistor.sym drops
// R1's pins; the gate suppresses the design's dangles rather than emit phantoms.
func TestDanglingSuppressedOnUnresolved(t *testing.T) {
	failOpen := func(string) ([]byte, error) { return nil, os.ErrNotExist }
	d, err := ReadWithSymbols(bytes.NewReader(readFixture(t, "dangle.sch")), "dangle.sch", failOpen)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(d.GetInputDiagnostics().GetDanglingEndpoints()); n != 0 {
		t.Errorf("unresolved symbol must suppress all dangles; got %d", n)
	}
}

// TestPinDirectionsAndExternal (WS1-021): gEDA pintype maps to pin directions (pwr ->
// power_in), and a power/ground supply symbol marks its net External — the two changes
// that make power rules reachable and keep them quiet on tapped rails.
func TestPinDirectionsAndExternal(t *testing.T) {
	// pwr_untapped: two ICs share a VCC net via pintype=pwr pins, no power symbol.
	d, err := ReadWithSymbols(bytes.NewReader(readFixture(t, "pwr_untapped.sch")), "x.sch", testOpener(t))
	if err != nil {
		t.Fatal(err)
	}
	var ic *ir.PartType
	for _, lib := range d.Libraries {
		for _, p := range lib.Parts {
			if p.Name == "ic" {
				ic = p
			}
		}
	}
	if ic == nil {
		t.Fatal("ic part type missing")
	}
	dirs := map[string]ir.PinDirection{}
	for _, p := range ic.Pins {
		dirs[p.Designator] = p.Direction
	}
	if dirs["1"] != ir.PinDirection_PIN_DIRECTION_POWER_IN {
		t.Errorf("pin 1 (pintype=pwr) dir = %v, want POWER_IN", dirs["1"])
	}
	if dirs["2"] != ir.PinDirection_PIN_DIRECTION_INPUT {
		t.Errorf("pin 2 (pintype=in) dir = %v, want INPUT", dirs["2"])
	}
	// The untapped VCC net carries the two power_in pins and is NOT external.
	for _, n := range d.Nets {
		if n.Name == "VCC" && n.Attributes["external"] == "true" {
			t.Error("untapped VCC must not be external (no power symbol)")
		}
	}

	// pwr_tapped: same rail with a vcc supply symbol -> External.
	dt, err := ReadWithSymbols(bytes.NewReader(readFixture(t, "pwr_tapped.sch")), "x.sch", testOpener(t))
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, n := range dt.Nets {
		if n.Name == "VCC" {
			found = true
			if n.Attributes["external"] != "true" {
				t.Errorf("tapped VCC attrs = %v, want external=true (supply symbol)", n.Attributes)
			}
		}
	}
	if !found {
		t.Error("VCC net missing on tapped design")
	}
}

// TestNetTaps (WS1-032): a component's net=NAME:pin attributes connect those pins to the named net
// with no drawn wire. U1 carries TWO net= lines (net=GND:2 and net=PWR:1 — the map-collapse case),
// and U1 + U2 both tap GND, so GND unites their pin 2s across the design without a wire.
func TestNetTaps(t *testing.T) {
	d, err := ReadWithSymbols(bytes.NewReader(readFixture(t, "nettap.sch")), "nettap.sch", testOpener(t))
	if err != nil {
		t.Fatal(err)
	}
	if got := connsOf(d, "GND"); !equalSet(got, []string{"U1.2", "U2.2"}) {
		t.Errorf("GND = %v, want [U1.2 U2.2] (both net=GND:2 taps merge by name)", got)
	}
	if got := connsOf(d, "PWR"); !equalSet(got, []string{"U1.1"}) {
		t.Errorf("PWR = %v, want [U1.1] (U1's second net= line, not collapsed by the attr map)", got)
	}
}

// TestVoltageRailSymbol (WS1-032): a voltage-rail symbol outside the conventional gnd/vcc/vdd/vss
// set (here 3.3V-plus-1.sym) is recognized as a power symbol, so it is not dropped for lacking a
// ref-des; its symbol-level net= names the rail and the tap marks it External.
func TestVoltageRailSymbol(t *testing.T) {
	d, err := ReadWithSymbols(bytes.NewReader(readFixture(t, "rail.sch")), "rail.sch", testOpener(t))
	if err != nil {
		t.Fatal(err)
	}
	var n *ir.Net
	for _, net := range d.Nets {
		if net.Name == "+3.3V" {
			n = net
		}
	}
	if n == nil {
		t.Fatalf("+3.3V rail net missing; nets = %v", netNames(d))
	}
	if n.Attributes["external"] != "true" {
		t.Errorf("+3.3V attrs = %v, want external=true (supply symbol)", n.Attributes)
	}
	if got := connsOf(d, "+3.3V"); !equalSet(got, []string{"U1.1"}) {
		t.Errorf("+3.3V = %v, want [U1.1]", got)
	}
}

// TestSlotting (WS1-032): a multi-gate package (numslots=2, slotdef rows) placed twice with a
// shared refdes and slot=1/slot=2 folds into ONE Component with a section per gate, and each
// gate's drawn pins remap to that slot's physical package pins. Without the remap both gates
// resolve slot-1's drawn numbers (1,2), so slot-2's terminals mis-key onto U1.1/U1.2 (the
// pin-net-conflict that fired on gTAG's U20).
func TestSlotting(t *testing.T) {
	d, err := ReadWithSymbols(bytes.NewReader(readFixture(t, "slotted.sch")), "slotted.sch", testOpener(t))
	if err != nil {
		t.Fatal(err)
	}
	if got := refs(d); !equalSet(got, []string{"U1"}) {
		t.Fatalf("components = %v, want a single folded U1", got)
	}
	if n := len(d.Components[0].Sections); n != 2 {
		t.Errorf("U1 sections = %d, want 2 (one per gate)", n)
	}
	// slot 1 -> physical pins 1,2 ; slot 2 -> physical pins 3,4 (slotdef=2:3,4).
	want := map[string][]string{
		"IN1":  {"U1.1"}, // slot 1 input pin (drawn pinseq 1 -> physical 1)
		"IN2":  {"U1.3"}, // slot 2 input pin (drawn pinseq 1 -> physical 3, NOT 1)
		"OUT2": {"U1.4"}, // slot 2 output pin (drawn pinseq 2 -> physical 4, NOT 2)
	}
	for name, conns := range want {
		if got := connsOf(d, name); !equalSet(got, conns) {
			t.Errorf("net %q = %v, want %v", name, got, conns)
		}
	}
}

// TestUnannotatedComponents (agni issue 311): gEDA keeps a placeholder-designated part, so it is
// the layer that has to report one. `<prefix>?` is gEDA's own convention — the symbol libraries in
// this testdata ship `refdes=R?` and `refdes=U?` as their template value — and the reader kept
// those silently.
//
// The fixture covers both halves of this reader's grouping fork, because they arrive at the entry
// differently. Two unslotted `R?` symbols become two ir.Components sharing a designator; two
// slotted `U?` gates fold into one Component with two sections. Either way the diagnostic is one
// entry per PLACEHOLDER carrying every placement, which is what makes "2 parts are still called R?"
// the reviewable fact rather than a number that depends on how the source spelled it.
func TestUnannotatedComponents(t *testing.T) {
	d, err := ReadWithSymbols(bytes.NewReader(readFixture(t, "unannotated.sch")), "unannotated.sch", testOpener(t))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, u := range d.GetInputDiagnostics().GetUnannotatedComponents() {
		got[u.GetRefDes()] = len(u.GetInstances())
	}
	want := map[string]int{"R?": 2, "U?": 2}
	if len(got) != len(want) {
		t.Fatalf("unannotated designators = %v, want %v", got, want)
	}
	for ref, n := range want {
		if got[ref] != n {
			t.Errorf("%q carries %d placements, want %d (have %v)", ref, got[ref], n, got)
		}
	}
	// The parts are kept, not dropped: unannotated circuitry is still circuitry.
	if r := refs(d); !equalSet(r, []string{"R?", "R?", "R1", "U?"}) {
		t.Errorf("components = %v, want the two R? placements, R1 and the folded U?", r)
	}
}

// TestUnannotatedWithoutSymbols pins the wiring on the OTHER entry path. Plain Read takes no symbol
// opener, and on that path this reader leaves InputDiagnostics nil unless the sheet has a bus, so a
// signal attached inside the resolving branch would be silently absent for every caller who does
// not supply a library.
func TestUnannotatedWithoutSymbols(t *testing.T) {
	d, err := Read(bytes.NewReader(readFixture(t, "unannotated.sch")), "unannotated.sch")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(d.GetInputDiagnostics().GetUnannotatedComponents()); n != 2 {
		t.Errorf("unannotated designators = %d, want 2 without a symbol library too", n)
	}
}

// TestRefDesCollisions (agni issue 309): gEDA states the gate, so it can tell a duplicated
// designator from the legitimate multi-gate case, and it now says so. Before this the reader
// emitted no collision at all and `duplicate-ref-des` read as a clean pass on every gEDA design.
//
// The fixture carries both duplicate shapes and both innocent ones, because the rule is only
// meaningful if it separates them:
//
//	R1   twice, unslotted        -> duplicate (two separate parts wearing one name)
//	U2   twice, both slot=1      -> duplicate (one gate claimed twice)
//	U1   slot=1 + slot=2         -> LEGITIMATE, the case the whole slot mechanism exists for
//	R2   once                    -> clean
//	R?   twice                   -> not a claimed name; unannotated-components reports those
func TestRefDesCollisions(t *testing.T) {
	d, err := ReadWithSymbols(bytes.NewReader(readFixture(t, "dup_refdes.sch")), "dup_refdes.sch", testOpener(t))
	if err != nil {
		t.Fatal(err)
	}

	got := map[string]int{}
	for _, c := range d.GetInputDiagnostics().GetRefDesCollisions() {
		got[c.GetRefDes()] = len(c.GetInstances())
	}
	want := map[string]int{"R1": 2, "U2": 2}
	if len(got) != len(want) {
		t.Fatalf("collisions = %v, want %v (U1's two gates are one part, R? is not a name)", got, want)
	}
	for ref, n := range want {
		if got[ref] != n {
			t.Errorf("collision %s = %d instances, want %d", ref, got[ref], n)
		}
	}

	// The declaration is the other half of the fix, and it is what a clean design carries too: it
	// says the reader LOOKED, so an empty list means "none" rather than "nobody asked".
	if !slices.Contains(d.GetInputDiagnostics().GetSupplied(), "ref_des_collisions") {
		t.Error("supplied does not name ref_des_collisions, so the rule gates itself off on gEDA")
	}
}

// A design with no duplicates still declares that it was checked. This is the assertion that would
// have caught the original bug: without it, "no collisions" and "never looked" are the same value.
func TestRefDesCollisionsDeclaredOnCleanRead(t *testing.T) {
	d, err := ReadWithSymbols(bytes.NewReader(readFixture(t, "divider.sch")), "divider.sch", testOpener(t))
	if err != nil {
		t.Fatal(err)
	}
	if n := len(d.GetInputDiagnostics().GetRefDesCollisions()); n != 0 {
		t.Fatalf("divider.sch has %d collisions, want a clean read", n)
	}
	if !slices.Contains(d.GetInputDiagnostics().GetSupplied(), "ref_des_collisions") {
		t.Error("a clean read must still declare it looked")
	}
}
