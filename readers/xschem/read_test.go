package xschem

import (
	"bytes"
	"os"
	"path/filepath"
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

// testOpener resolves symbols from the testdata directory.
func testOpener(t *testing.T) SymbolOpener {
	t.Helper()
	return func(symref string) ([]byte, error) {
		return os.ReadFile(filepath.Join("testdata", filepath.Base(symref)))
	}
}

func TestIsXschem(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"v {xschem version=3.4.4 file_version=1.2}\n", true},
		{"* a banner comment\n\nv {xschem version=3.4.4}\n", true},
		{"v 20200319 2\n", false}, // gEDA
		{"(kicad_sch (version 20230121)", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsXschem([]byte(c.in)); got != c.want {
			t.Errorf("IsXschem(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestReadStructural(t *testing.T) {
	// No opener: components and named nets, but no pin connections.
	d, err := Read(bytes.NewReader(readFixture(t, "divider.sch")), "divider.sch")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if d.SourceFormat != "xschem" {
		t.Errorf("source format = %q, want xschem", d.SourceFormat)
	}
	if len(d.Components) != 2 {
		t.Fatalf("components = %d, want 2 (R1,R2)", len(d.Components))
	}
	if d.Components[0].RefDes != "R1" || d.Components[1].RefDes != "R2" {
		t.Errorf("refdes = %q,%q; want R1,R2", d.Components[0].RefDes, d.Components[1].RefDes)
	}
	// R1's multi-line {props} block must still parse its value.
	if got := d.Components[0].Sections[0].Attributes["value"]; got != "1k" {
		t.Errorf("R1 value = %q, want 1k (multi-line props not parsed?)", got)
	}
	if got := netNames(d); !equalSet(got, []string{"IN", "MID", "OUT", "0"}) {
		t.Errorf("net names = %v, want IN,MID,OUT,0", got)
	}
	for _, n := range d.Nets {
		if len(n.Connections) != 0 {
			t.Errorf("net %q has connections without a symbol library", n.Name)
		}
	}
}

func TestReadWithSymbols(t *testing.T) {
	d, err := ReadWithSymbols(bytes.NewReader(readFixture(t, "divider.sch")), "divider.sch", testOpener(t))
	if err != nil {
		t.Fatalf("ReadWithSymbols: %v", err)
	}
	want := map[string][]string{
		"IN":  {"R1.1"},
		"MID": {"R1.2", "R2.1"},
		"OUT": {"R2.2"},
		"0":   {}, // gnd tap names the net but nothing else lands on it
	}
	for name, conns := range want {
		got := connsOf(d, name)
		if !equalSet(got, conns) {
			t.Errorf("net %q = %v, want %v", name, got, conns)
		}
	}
	// The resistor PartType picks up its two pins from the resolved symbol.
	if pt := partType(d, "res"); pt == nil {
		t.Fatal("no res PartType")
	} else if len(pt.Pins) != 2 {
		t.Errorf("res pins = %d, want 2", len(pt.Pins))
	}
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

func partType(d *ir.Design, name string) *ir.PartType {
	for _, lib := range d.Libraries {
		for _, pt := range lib.Parts {
			if pt.Name == name {
				return pt
			}
		}
	}
	return nil
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
// endpoint IN THE GEOMETRY FRAME (native coords, not the scaled netgraph grid), but only
// when every symbol resolves — an unresolved symbol drops its pins and would fabricate
// dangles, so the whole design's dangles are suppressed.
func TestDanglingEndpoints(t *testing.T) {
	d, err := ReadWithSymbols(bytes.NewReader(readFixture(t, "dangle.sch")), "dangle.sch", testOpener(t))
	if err != nil {
		t.Fatal(err)
	}
	dangles := d.GetInputDiagnostics().GetDanglingEndpoints()
	if len(dangles) != 1 {
		t.Fatalf("dangles = %d, want 1: %+v", len(dangles), dangles)
	}
	// Geometry frame: the wire end is at native (100,90). The netgraph grid scales by 2,
	// so a missing unquant would report (200,180).
	if dangles[0].X != 100 || dangles[0].Y != 90 {
		t.Errorf("dangle at (%d,%d), want geometry-frame (100,90)", dangles[0].X, dangles[0].Y)
	}
	if dangles[0].Prov.GetSourceFile() != "dangle.sch" || dangles[0].Prov.GetNativeId() != "" {
		t.Errorf("prov = %+v, want source-only (xschem wires carry no id)", dangles[0].Prov)
	}
}

// TestDanglingSuppressedOnUnresolved: an opener that cannot resolve res.sym drops R1's
// pins, so the wire ends would ALL read as dangling — the gate suppresses the entire set
// rather than emit phantoms.
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

// TestPinDirectionsAndExternal (WS1-021): xschem pin `dir` maps to directions (res.sym
// pins are dir=inout -> INOUT), and a supply symbol (vdd) marks its net External while a
// plain net label (lab_pin) does not.
func TestPinDirectionsAndExternal(t *testing.T) {
	d, err := ReadWithSymbols(bytes.NewReader(readFixture(t, "power.sch")), "x.sch", testOpener(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, lib := range d.Libraries {
		for _, p := range lib.Parts {
			if p.Name == "xschem:R" || p.Name == "R" {
				for _, pin := range p.Pins {
					if pin.Direction != ir.PinDirection_PIN_DIRECTION_INOUT {
						t.Errorf("R pin %s dir = %v, want INOUT (dir=inout)", pin.Designator, pin.Direction)
					}
				}
			}
		}
	}
	ext := map[string]string{}
	for _, n := range d.Nets {
		ext[n.Name] = n.Attributes["external"]
	}
	if ext["VDD"] != "true" {
		t.Errorf("VDD (vdd supply symbol) external = %q, want true", ext["VDD"])
	}
	if ext["SIG"] == "true" {
		t.Errorf("SIG (lab_pin, a plain net label) must not be external")
	}
}

// TestUnannotatedComponents (agni issue 311): this reader keeps a placeholder-designated component,
// which makes it the layer that has to report one.
//
// The caveat belongs on the record. Nothing attests that xschem SHIPS placeholders the way gEDA's
// refdes=R? templates do, because xschem assigns an instance name from the symbol template on
// placement, so this fixture is hand-authored rather than drawn from the format's conventions. It
// is here because the reader's own assumption depends on the answer: the instance name is this
// format's provenance native id and is documented as unique within a schematic, and two components
// sharing "R?" break that quietly. Each pin 1 then collapses onto the same ("R?", "1") key in the
// net graph, which is the degradation internal/refdes exists to keep out of the index.
//
// Note this reader does NOT fold by designator, so the two "R?" placements stay two components. The
// diagnostic is still one entry, because it groups by designator before walking sections.
func TestUnannotatedComponents(t *testing.T) {
	d, err := ReadWithSymbols(bytes.NewReader(readFixture(t, "unannotated.sch")), "unannotated.sch", testOpener(t))
	if err != nil {
		t.Fatal(err)
	}
	un := d.GetInputDiagnostics().GetUnannotatedComponents()
	if len(un) != 1 || un[0].GetRefDes() != "R?" {
		t.Fatalf("unannotated components = %+v, want one R? entry", un)
	}
	if n := len(un[0].GetInstances()); n != 2 {
		t.Errorf("R? carries %d placements, want 2", n)
	}
	var refs []string
	for _, c := range d.Components {
		refs = append(refs, c.RefDes)
	}
	if !equalSet(refs, []string{"R1", "R?", "R?"}) {
		t.Errorf("components = %v, want R1 and both R? placements kept", refs)
	}
}

// TestUnannotatedWithoutSymbols pins the wiring on the other entry path. Plain Read supplies no
// opener, and on that path InputDiagnostics stays nil unless the sheet has a bus, so a signal
// attached inside the resolving branch is absent for every caller without a symbol library.
func TestUnannotatedWithoutSymbols(t *testing.T) {
	d, err := Read(bytes.NewReader(readFixture(t, "unannotated.sch")), "unannotated.sch")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(d.GetInputDiagnostics().GetUnannotatedComponents()); n != 1 {
		t.Errorf("unannotated designators = %d, want 1 without a symbol library too", n)
	}
}
