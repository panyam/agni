package ipc2581

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// readFixture loads a testdata file, failing the test if it is missing.
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func TestRead(t *testing.T) {
	d, err := Read(bytes.NewReader(readFixture(t, "board.xml")), "test.xml")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if d.SourceFormat != "ipc-2581" || d.IrVersion != "0" {
		t.Errorf("source_format=%q ir_version=%q, want ipc-2581 / 0", d.SourceFormat, d.IrVersion)
	}

	// Netlist tier: one section per placed component (part_ref = packageRef), footprint_ref =
	// packageRef, so section-aware consumers match the EDIF/schematic readers.
	if len(d.Components) != 3 {
		t.Fatalf("components = %d, want 3", len(d.Components))
	}
	r1 := findComponent(d, "R1")
	if r1 == nil || len(r1.Sections) != 1 || r1.FootprintRef != "R0603" {
		t.Fatalf("R1 = %v, want 1 section + footprint_ref R0603", r1)
	}
	if s := r1.Sections[0]; s.PartRef != "R0603" {
		t.Errorf("R1 section part_ref = %q, want R0603 (package)", s.PartRef)
	}
	// NonstandardAttribute VALUE lands on the conventional "Value" key (WS1-031).
	if r1.Attributes["Value"] != "10K" {
		t.Errorf("R1 Value = %q, want 10K (from NonstandardAttribute)", r1.Attributes["Value"])
	}
	if got := connKeys(findNet(d, "GND")); !eqSet(got, []string{"R1.2", "U1.4"}) {
		t.Errorf("GND connections = %v, want [R1.2 U1.4]", got)
	}

	// Physical tier — the point of this reader.
	if len(d.Footprints) != 2 {
		t.Errorf("footprints = %d, want 2", len(d.Footprints))
	}
	if len(d.Layers) != 2 {
		t.Fatalf("layers = %d, want 2", len(d.Layers))
	}
	top := findLayer(d, "TOP")
	if top == nil || top.Function != ir.LayerFunction_LAYER_FUNCTION_SIGNAL {
		t.Errorf("TOP layer = %v, want function SIGNAL (mapped from CONDUCTOR)", top)
	}
	if top != nil && top.Attributes["side"] != "TOP" {
		t.Errorf("TOP layer side attr = %q, want TOP", top.Attributes["side"])
	}
	if d.Stackup == nil || len(d.Stackup.Layers) != 2 {
		t.Fatalf("stackup layers = %v, want 2", d.Stackup)
	}
	if sl := d.Stackup.Layers[0]; sl.ThicknessNm != 35000 || sl.Material != "COPPER" {
		t.Errorf("stackup layer 0 = %+v, want thickness_nm 35000 (0.035mm) material COPPER", sl)
	}
	if len(d.Bom) != 1 {
		t.Fatalf("bom lines = %d, want 1", len(d.Bom))
	}
	if b := d.Bom[0]; !eqSet(b.RefDes, []string{"R1", "R2"}) || b.Quantity != 2 || b.Mpn != "RES-10K" {
		t.Errorf("bom[0] = %+v, want ref_des [R1 R2] qty 2 mpn RES-10K", b)
	}
}

func findComponent(d *ir.Design, ref string) *ir.Component {
	for _, c := range d.Components {
		if c.RefDes == ref {
			return c
		}
	}
	return nil
}

func findNet(d *ir.Design, name string) *ir.Net {
	for _, n := range d.Nets {
		if n.Name == name {
			return n
		}
	}
	return nil
}

func findLayer(d *ir.Design, name string) *ir.Layer {
	for _, l := range d.Layers {
		if l.Name == name {
			return l
		}
	}
	return nil
}

func connKeys(n *ir.Net) []string {
	if n == nil {
		return nil
	}
	var out []string
	for _, c := range n.Connections {
		out = append(out, c.ComponentRef+"."+c.PinRef)
	}
	return out
}

func eqSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	m := map[string]int{}
	for _, g := range got {
		m[g]++
	}
	for _, w := range want {
		m[w]--
	}
	for _, v := range m {
		if v != 0 {
			return false
		}
	}
	return true
}

// TestDeclaredNetRole covers WS1-051: IPC-2581 states a net's role outright on
// LogicalNet/@netClass, so the reader translates that closed enum into the engine's role
// vocabulary at the edge and leaves the source term beside it. The mapping is deliberately
// partial — only GROUND and POWER name a role the engine has — and an unmapped term must
// yield no role rather than a guess.
func TestDeclaredNetRole(t *testing.T) {
	d, err := Read(bytes.NewReader(readFixture(t, "board.xml")), "board.xml")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ net, raw, role string }{
		{"GND", "GROUND", classify.NetRoleGround},
		{"VCC", "POWER", classify.NetRoleRail},
		{"N$17", "GROUND", classify.NetRoleGround}, // the point: the name says nothing
		{"SIGNAL_ONLY", "SIGNAL", ""},              // maps to no role; must not invent one
	} {
		n := findNet(d, tc.net)
		if n == nil {
			t.Errorf("net %q not found", tc.net)
			continue
		}
		if got := n.Attributes["netclass_raw"]; got != tc.raw {
			t.Errorf("net %q netclass_raw = %q, want %q", tc.net, got, tc.raw)
		}
		if got := n.Attributes[classify.AttrDeclaredRole]; got != tc.role {
			t.Errorf("net %q %s = %q, want %q", tc.net, classify.AttrDeclaredRole, got, tc.role)
		}
	}
}

// TestSkippedRefDes (agni issue 311): IPC-2581 is a board format, so it declines an unannotated
// component the way the KiCad board reader does rather than recording it as a diagnostic. Only a
// reader that KEEPS the part owes that, and a REF** on a fabrication artifact is usually a fiducial
// or a mechanical part rather than something anybody is going to buy.
//
// The designator-less component is in the same test because it is the same guard: the geometry pass
// already dropped those, so the netlist used to carry components the board had no placement for.
// The last assertion is the one that matters — the two tiers agree on the component set.
func TestSkippedRefDes(t *testing.T) {
	d, err := Read(bytes.NewReader(readFixture(t, "board.xml")), "board.xml")
	if err != nil {
		t.Fatal(err)
	}
	refs := map[string]bool{}
	for _, c := range d.Components {
		refs[c.RefDes] = true
	}
	for _, bad := range []string{"REF**", "C?1845", ""} {
		if refs[bad] {
			t.Errorf("netlist kept the component %q, which names no part", bad)
		}
	}
	for _, want := range []string{"R1", "R2", "U1"} {
		if !refs[want] {
			t.Errorf("netlist dropped the real component %q; have %v", want, refs)
		}
	}
	// A connection to a skipped component would claim a pin on a part that is not in the design.
	// Asserted against the component set rather than against skipRefDes: a test that reaches for
	// the production predicate to decide what counts as a failure cannot fail when that predicate
	// is what broke.
	for _, n := range d.Nets {
		for _, c := range n.Connections {
			if !refs[c.ComponentRef] {
				t.Errorf("net %q keeps a connection to %q, which is not a component", n.Name, c.ComponentRef)
			}
		}
	}

	g, err := ReadBoardGeometry(bytes.NewReader(readFixture(t, "board_geom.xml")), "board_geom.xml")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range g.Placements {
		if p.RefDes == "REF**" || p.RefDes == "" {
			t.Errorf("board geometry kept the placement %q, which names no part", p.RefDes)
		}
	}
	for _, gr := range g.Graphics {
		if gr.GetRefDes() == "REF**" {
			t.Error("board geometry kept body graphics for an unannotated component")
		}
	}
}

// TestRefDesCollisions (agni issue 309): a board states one placement per physical part and has no
// gate construct to group several under one designator, so a repeated refDes is unambiguous. This
// reader emitted nothing before, and `duplicate-ref-des` therefore read as a clean pass on every
// IPC-2581 design rather than as a question nobody asked.
func TestRefDesCollisions(t *testing.T) {
	d, err := Read(bytes.NewReader(readFixture(t, "board_dup_refdes.xml")), "board_dup_refdes.xml")
	if err != nil {
		t.Fatal(err)
	}
	cols := d.GetInputDiagnostics().GetRefDesCollisions()
	if len(cols) != 1 || cols[0].GetRefDes() != "R2" {
		t.Fatalf("collisions = %+v, want one for R2", cols)
	}
	if n := len(cols[0].GetInstances()); n != 2 {
		t.Errorf("R2 instances = %d, want 2", n)
	}
}

// The clean read is the assertion that would have caught the original bug: an empty list has to
// come with the declaration, or it is indistinguishable from a reader that never looked.
func TestRefDesCollisionsDeclaredOnCleanRead(t *testing.T) {
	d, err := Read(bytes.NewReader(readFixture(t, "board.xml")), "board.xml")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(d.GetInputDiagnostics().GetRefDesCollisions()); n != 0 {
		t.Fatalf("board.xml has %d collisions, want a clean read", n)
	}
	if !slices.Contains(d.GetInputDiagnostics().GetSupplied(), "ref_des_collisions") {
		t.Error("a clean read must still declare it looked")
	}
}

// A placeholder designator is annotation state, not a claimed name, and this reader drops those
// components entirely. Two fiducials wearing "REF**" are not two parts fighting over one name.
func TestRefDesCollisionsIgnorePlaceholders(t *testing.T) {
	d, err := Read(bytes.NewReader(readFixture(t, "board.xml")), "board.xml")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range d.GetInputDiagnostics().GetRefDesCollisions() {
		if c.GetRefDes() == "REF**" || c.GetRefDes() == "C?1845" {
			t.Errorf("placeholder %q reported as a collision", c.GetRefDes())
		}
	}
}
