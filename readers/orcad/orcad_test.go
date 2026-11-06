package orcad

import (
	"encoding/binary"
	"sort"
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// A test design is built as the parsed structures, so each test states the one rule it checks.
// The byte layouts are held to a real design by the Clay R6 tests in readers/formats.

func resistor(db uint32, ref string, a, b point) placedPart {
	return placedPart{db: db, cacheKey: "R.Normal", ref: ref, pkg: "R", pins: []placedPin{{index: 1, at: a}, {index: 2, at: b}}}
}

func testDesign(name string, pages ...*page) *dsn {
	return &dsn{
		name: name,
		lib:  &library{major: 3, minor: 2},
		cache: &cache{
			parts:    map[string]*libPart{"R.Normal": {pins: []symPin{{slot: 0, name: "A"}, {slot: 1, name: "B"}}}},
			packages: map[string]*pkg{"R": {footprint: "R0402", devices: []pkgDevice{{pins: []string{"1", "2"}}}}},
		},
		folders: map[string][]*page{"schematic1": pages},
		root:    "schematic1",
	}
}

func netsOf(t *testing.T, d *ir.Design) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, n := range d.GetNets() {
		var pins []string
		for _, c := range n.GetConnections() {
			pins = append(pins, c.GetComponentRef()+"."+c.GetPinRef())
		}
		sort.Strings(pins)
		out[n.GetName()] = strings.Join(pins, " ")
	}
	return out
}

func TestAPinJoinsTheWireItsPositionSitsOn(t *testing.T) {
	pg := &page{
		netName: map[uint32][]string{7: {"SIG"}},
		wires:   []wire{{net: 7, a: point{0, 0}, b: point{100, 0}}},
		parts: []placedPart{
			resistor(1, "R1", point{0, 0}, point{0, 50}),   // pin 1 on the wire's end
			resistor(2, "R2", point{40, 0}, point{40, 50}), // pin 1 mid-span
			resistor(3, "R3", point{40, 1}, point{40, 60}), // off the wire by one unit
		},
	}
	d, err := flatten(testDesign("t.dsn", pg), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := netsOf(t, d)["SIG"]; got != "R1.1 R2.1" {
		t.Errorf("SIG holds %q, want R1.1 and R2.1 and not R3.1", got)
	}
}

func TestAnUnnamedNetIsNumberedFromItsLowestWire(t *testing.T) {
	pg := &page{
		netName: map[uint32][]string{},
		wires: []wire{
			{id: 409, net: 7, a: point{0, 0}, b: point{50, 0}},
			{id: 299, net: 7, a: point{50, 0}, b: point{100, 0}},
		},
		parts: []placedPart{
			resistor(1, "R1", point{0, 0}, point{0, 50}),
			resistor(2, "R2", point{100, 0}, point{100, 50}),
		},
	}
	d, err := flatten(testDesign("t.dsn", pg), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := netsOf(t, d)["N00299"]; got != "R1.1 R2.1" {
		t.Errorf("nets %v, want R1.1 and R2.1 on N00299, after the lower wire id", netsOf(t, d))
	}
}

func TestTheOccurrenceTreeNamesAPartOverItsPlacement(t *testing.T) {
	pg := &page{netName: map[uint32][]string{}, wires: []wire{{net: 1, a: point{0, 0}, b: point{10, 0}}},
		parts: []placedPart{resistor(42, "R?", point{0, 0}, point{10, 0})}}
	top := testDesign("t.dsn", pg)
	top.occ = &scope{occ: map[uint32]*occurrence{42: {target: 42, ref: "R17"}}}
	d, err := flatten(top, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.GetComponents()) != 1 || d.GetComponents()[0].GetRefDes() != "R17" {
		t.Errorf("components %v, want R17 from the occurrence tree", d.GetComponents())
	}
}

func TestPowerSymbolsJoinAcrossBlocksAndAPortBindsItsOwn(t *testing.T) {
	// The top draws GND and GND_EARTH as power symbols. Block A draws GND and has no GND port, so its
	// GND is the design's. Block B draws GND beside a GND port, so its GND is whatever the parent
	// wires that port to, here GND_EARTH.
	top := &page{
		netName: map[uint32][]string{},
		wires: []wire{
			{net: 1, a: point{0, 0}, b: point{0, 10}},     // to R1.1 and the GND symbol
			{net: 2, a: point{100, 0}, b: point{100, 10}}, // to block B's GND pin and GND_EARTH
		},
		parts: []placedPart{resistor(1, "R1", point{0, 10}, point{0, 20})},
		symbols: []pageSymbol{
			{kind: symGlobal, name: "GND", lo: point{-5, -5}, hi: point{5, 0}},
			{kind: symGlobal, name: "GND_EARTH", lo: point{95, -5}, hi: point{105, 0}},
		},
		blocks: []block{
			{db: 10, ref: "A", file: "a.dsn"},
			{db: 11, ref: "B", file: "b.dsn", pinNames: []string{"GND"}, pins: []placedPin{{index: 1, at: point{100, 10}}}},
		},
	}
	childPage := func(port bool) *page {
		pg := &page{
			netName: map[uint32][]string{},
			wires:   []wire{{net: 5, a: point{0, 0}, b: point{0, 10}}},
			parts:   []placedPart{resistor(1, "R?", point{0, 10}, point{0, 20})},
			symbols: []pageSymbol{{kind: symGlobal, name: "GND", lo: point{-5, -5}, hi: point{5, 0}}},
		}
		if port {
			pg.symbols = append(pg.symbols, pageSymbol{kind: symPort, name: "GND", lo: point{0, 5}, hi: point{20, 5}})
		}
		return pg
	}
	a, b := testDesign("a.dsn", childPage(false)), testDesign("b.dsn", childPage(true))
	td := testDesign("top.dsn", top)
	td.occ = &scope{occ: map[uint32]*occurrence{
		10: {target: 10, nested: &scope{occ: map[uint32]*occurrence{1: {target: 1, ref: "R2"}}}},
		11: {target: 11, nested: &scope{occ: map[uint32]*occurrence{1: {target: 1, ref: "R3"}}}},
	}}
	open := func(from, ref string) (string, []byte, error) { return ref, nil, nil }
	f := newFlattener(td, open)
	f.designs["a.dsn"], f.designs["b.dsn"] = a, b
	if err := f.walk(td, td.root, "", td.occ, 0, map[string]bool{}); err != nil {
		t.Fatal(err)
	}
	got := netsOf(t, f.design(td))
	if got["GND"] != "R1.1 R2.1" {
		t.Errorf("GND holds %q, want R1.1 and block A's R2.1", got["GND"])
	}
	if got["GND_EARTH"] != "R3.1" {
		t.Errorf("GND_EARTH holds %q, want block B's R3.1, which its GND port takes there", got["GND_EARTH"])
	}
}

func TestACornerOfASymbolsBoxIsNotItsConnection(t *testing.T) {
	s := pageSymbol{lo: point{0, 0}, hi: point{20, 10}}
	for _, p := range []point{{10, 0}, {0, 5}, {20, 5}, {10, 10}, {10, 5}} {
		if !s.covers(p) {
			t.Errorf("%v is on the box and not a corner, so it should connect", p)
		}
	}
	for _, p := range []point{{0, 0}, {20, 0}, {0, 10}, {20, 10}, {21, 5}} {
		if s.covers(p) {
			t.Errorf("%v is a corner or outside the box, so it should not connect", p)
		}
	}
}

func TestAPackShortJoinsTheNamedPins(t *testing.T) {
	part := resistor(1, "U1", point{0, 0}, point{500, 500})
	part.props = map[string]string{"PACK_SHORT": "(A,B)"}
	pg := &page{netName: map[uint32][]string{}, wires: []wire{{net: 3, a: point{0, 0}, b: point{0, 10}}},
		parts: []placedPart{part, resistor(2, "R2", point{0, 10}, point{0, 20})}}
	d, err := flatten(testDesign("t.dsn", pg), nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, pins := range netsOf(t, d) {
		if pins == "R2.1 U1.1 U1.2" {
			found = true
		}
	}
	if !found {
		t.Errorf("nets %v, want U1's two pins shorted onto R2.1's net", netsOf(t, d))
	}
	if g := packShorts("(VDD,VDD1,VDD2)(GND,GND1,GND2)"); len(g) != 2 || strings.Join(g[1], ",") != "GND,GND1,GND2" {
		t.Errorf("groups %v", g)
	}
}

func TestANotConnectedPinStaysOffEveryNet(t *testing.T) {
	part := resistor(1, "R1", point{0, 0}, point{0, 10})
	part.pins[0].index = -1
	pg := &page{netName: map[uint32][]string{}, wires: []wire{{net: 3, a: point{0, 0}, b: point{0, 10}}},
		parts: []placedPart{part}}
	d, err := flatten(testDesign("t.dsn", pg), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, pins := range netsOf(t, d) {
		if strings.Contains(pins, "R1.1") {
			t.Errorf("R1.1 is flagged not connected and reads on %q", pins)
		}
	}
}

func TestABlockWhoseFileIsMissingIsListed(t *testing.T) {
	pg := &page{netName: map[uint32][]string{}, blocks: []block{{db: 9, ref: "IMU", file: `.\MPU9250\MPU9250.DSN`}}}
	d, err := flatten(testDesign("t.dsn", pg), nil)
	if err != nil {
		t.Fatal(err)
	}
	u := d.GetInputDiagnostics().GetUnexpandedHierarchy()
	if len(u) != 1 || u[0].GetName() != "IMU" || u[0].GetReason() != ir.UnexpandedReason_UNEXPANDED_REASON_FILE_MISSING ||
		u[0].GetProv().GetNativeId() != `.\MPU9250\MPU9250.DSN` {
		t.Errorf("unexpanded %v, want IMU listed with its file as missing", u)
	}
}

func TestADesignThatIsNotACompoundFileIsRefused(t *testing.T) {
	_, err := Read(strings.NewReader("(pcb board.dsn (parser (host_cad KiCad)))"), "board.dsn", nil)
	if err == nil || !strings.Contains(err.Error(), "SPECCTRA") {
		t.Errorf("err %v, want a SPECCTRA file refused by name", err)
	}
}

func TestALegacyLibraryIsRefusedByItsVersion(t *testing.T) {
	b := make([]byte, 64)
	copy(b, "OrCAD Windows Design")
	binary.LittleEndian.PutUint16(b[32:], 2)
	_, err := parseLibrary(b)
	if err == nil || !strings.Contains(err.Error(), "legacy format 2.0") {
		t.Errorf("err %v, want the legacy format refused by its version", err)
	}
	copy(b, "OrCAD Windows Library\x00")
	if _, err := parseLibrary(b); err == nil || !strings.Contains(err.Error(), "not an OrCAD Capture design") {
		t.Errorf("err %v, want a library file refused as not a design", err)
	}
}

func TestAFrameOfAnotherTypeMidChainIsAnError(t *testing.T) {
	// A type-20 wire has two long prefixes. The second here says type 21.
	b := []byte{20, 0, 0, 0, 0, 0, 0, 0, 0, 21, 0, 0, 0, 0, 0, 0, 0, 0}
	c := &cursor{b: b}
	c.frame()
	if c.err == nil || !strings.Contains(c.err.Error(), "another type") {
		t.Errorf("err %v, want the mismatched prefix refused", c.err)
	}
}
