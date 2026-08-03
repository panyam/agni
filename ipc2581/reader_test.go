package ipc2581

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

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
