package kicad

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

func TestReadPCB(t *testing.T) {
	d, err := Read(bytes.NewReader(readFixture(t, "pcb.kicad_pcb")), "test.kicad_pcb")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if d.SourceFormat != "kicad-pcb" {
		t.Errorf("source_format = %q, want kicad-pcb", d.SourceFormat)
	}
	if d.IrVersion != "0" {
		t.Errorf("ir_version = %q, want 0", d.IrVersion)
	}

	// Components: two physical parts, keyed by ref_des, each one section (one placed footprint =
	// one section) so section-aware consumers match the EDIF/schematic readers.
	if got := len(d.Components); got != 2 {
		t.Fatalf("components = %d, want 2", got)
	}
	r1 := findComponent(d, "R1")
	if r1 == nil {
		t.Fatal("component R1 not found")
	}
	if len(r1.Sections) != 1 {
		t.Fatalf("R1 sections = %d, want 1 (one placed footprint)", len(r1.Sections))
	}
	if s := r1.Sections[0]; s.PartRef != "Lib:R_0603" || s.LibraryRef != "Lib" {
		t.Errorf("R1 section part/lib = %q/%q, want Lib:R_0603/Lib (footprint)", s.PartRef, s.LibraryRef)
	}
	if r1.FootprintRef != "Lib:R_0603" {
		t.Errorf("R1 footprint_ref = %q, want Lib:R_0603", r1.FootprintRef)
	}
	if r1.Attributes["Value"] != "10k" {
		t.Errorf("R1 Value = %q, want 10k", r1.Attributes["Value"])
	}
	if r1.Prov.GetNativeId() != "aaaa-1111" || r1.Prov.GetNativeIdKind() != "kicad-uuid" {
		t.Errorf("R1 prov = %v, want native_id aaaa-1111 kind kicad-uuid", r1.Prov)
	}

	// Footprints (provisional tier), deduped by name.
	if got := len(d.Footprints); got != 2 {
		t.Errorf("footprints = %d, want 2", got)
	}

	// Nets: net-0 "" dropped; GND and VCC remain with the right membership.
	if got := len(d.Nets); got != 2 {
		t.Fatalf("nets = %d, want 2 (net-0 dropped)", got)
	}
	if got := connKeys(findNet(d, "GND")); !eqSet(got, []string{"R1.2"}) {
		t.Errorf("GND connections = %v, want [R1.2]", got)
	}
	if got := connKeys(findNet(d, "VCC")); !eqSet(got, []string{"C1.1", "R1.1"}) {
		t.Errorf("VCC connections = %v, want [C1.1 R1.1]", got)
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
