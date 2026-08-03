package kicad

import (
	"bytes"
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func TestReadProject(t *testing.T) {
	d, err := ReadProject(
		bytes.NewReader(readFixture(t, "project.kicad_sch")), bytes.NewReader(readFixture(t, "project.kicad_pcb")),
		"test.kicad_sch", "test.kicad_pcb", nil)
	if err != nil {
		t.Fatalf("ReadProject: %v", err)
	}
	if d.SourceFormat != "kicad" {
		t.Errorf("source_format = %q, want kicad", d.SourceFormat)
	}

	// Logical structure comes from the schematic.
	if findPartType(d, "test:DUAL") == nil {
		t.Error("part type test:DUAL missing (should come from schematic)")
	}

	// U1: sections from the schematic AND footprint_ref joined from the board.
	u1 := findComponent(d, "U1")
	if u1 == nil {
		t.Fatal("component U1 not found")
	}
	if len(u1.Sections) != 2 {
		t.Errorf("U1 sections = %d, want 2 (from schematic)", len(u1.Sections))
	}
	if u1.FootprintRef != "Pkg:SOIC-8" {
		t.Errorf("U1 footprint_ref = %q, want Pkg:SOIC-8 (joined from board)", u1.FootprintRef)
	}

	// Board-only component is carried over with its one board section (from the pcb reader).
	mh := findComponent(d, "MH1")
	if mh == nil {
		t.Fatal("board-only component MH1 not found")
	}
	if len(mh.Sections) != 1 {
		t.Errorf("MH1 sections = %d, want 1 (board placement)", len(mh.Sections))
	}

	// Connectivity comes from the board.
	if len(d.Nets) != 2 {
		t.Fatalf("nets = %d, want 2 (from board)", len(d.Nets))
	}
	if got := connKeys(findNet(d, "N1")); !eqSet(got, []string{"U1.1"}) {
		t.Errorf("N1 connections = %v, want [U1.1]", got)
	}
}

// TestProjectResolvesCrossSheet pins the WS1-017 semantics: external means "continues
// into something we did NOT read". A schematic-only project whose root has no sub-sheet
// references was read completely, so its power-symbol nets downgrade external -> global
// (rules fire; rail-ness stays queryable). The same file read as a bare .kicad_sch keeps
// the conservative marking (it could be one sheet of a larger design), and a root WITH
// sub-sheet references keeps it too (the netlist walk is root-only today).
func TestProjectResolvesCrossSheet(t *testing.T) {
	flat := `(kicad_sch (version 20250114) (generator "test")
	(lib_symbols
		(symbol "test:MCU" (property "Reference" "U")
			(symbol "MCU_1_1" (pin power_in line (at 0 0 0) (length 0) (name "VDD") (number "1"))))
		(symbol "power:VCC" (property "Reference" "#PWR") (property "Value" "VCC")
			(symbol "VCC_1_1" (pin power_in line (at 0 0 0) (length 0) (name "~") (number "1")))))
	(symbol (lib_id "test:MCU") (at 50.8 50.8 0) (unit 1) (uuid "u1")
		(property "Reference" "U1") (property "Value" "MCU"))
	(symbol (lib_id "power:VCC") (at 25.4 50.8 0) (unit 1) (uuid "pwr1")
		(property "Reference" "#PWR01") (property "Value" "VCC"))
	(wire (pts (xy 25.4 50.8) (xy 50.8 50.8)))
)`
	attrsOf := func(d *ir.Design, net string) map[string]string {
		for _, n := range d.Nets {
			if n.Name == net {
				return n.Attributes
			}
		}
		t.Fatalf("net %q not found", net)
		return nil
	}

	d, err := ReadProject(strings.NewReader(flat), nil, "flat.kicad_sch", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	a := attrsOf(d, "VCC")
	if a["external"] == "true" {
		t.Error("flat schematic-only project: external must downgrade (the whole design was read)")
	}
	if a["global"] != "true" {
		t.Error("flat schematic-only project: downgraded net should carry global=true")
	}

	bare, err := ReadSchematic(strings.NewReader(flat), "flat.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	if attrsOf(bare, "VCC")["external"] != "true" {
		t.Error("bare .kicad_sch keeps the conservative external marking")
	}

	withSub := strings.Replace(flat, "\n)", `
	(sheet (at 100 100) (uuid "sub1") (property "Sheetname" "amp") (property "Sheetfile" "amp.kicad_sch"))
)`, 1)
	d2, err := ReadProject(strings.NewReader(withSub), nil, "root.kicad_sch", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if attrsOf(d2, "VCC")["external"] != "true" {
		t.Error("root whose sub-sheet cannot be opened keeps external (partial walk)")
	}
}

// Degrade: with no board, the project reader returns the schematic IR alone (structure,
// no nets in this wireless fixture).
func TestReadProjectSchematicOnly(t *testing.T) {
	d, err := ReadProject(bytes.NewReader(readFixture(t, "project.kicad_sch")), nil, "test.kicad_sch", "", nil)
	if err != nil {
		t.Fatalf("ReadProject: %v", err)
	}
	if findComponent(d, "U1") == nil {
		t.Error("U1 missing")
	}
	if len(d.Nets) != 0 {
		t.Errorf("nets = %d, want 0 (no board)", len(d.Nets))
	}
}
