package kicad

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseSymLibTable(t *testing.T) {
	got := ParseSymLibTable(readFixture(t, "extlib-sym-lib-table"), "/proj")
	if got["ext"] != "/proj/ext.kicad_sym" {
		t.Errorf("table = %v, want ext -> /proj/ext.kicad_sym (${KIPRJMOD} expanded)", got)
	}
	if ParseSymLibTable([]byte("(not_a_table)"), "/p") != nil {
		t.Error("non-table input must parse to nil")
	}
}

func extOpen(t *testing.T) func(string) ([]byte, error) {
	return func(lib string) ([]byte, error) { return readFixture(t, lib+".kicad_sym"), nil }
}

// TestExternalSymbolResolution (WS1-016): a schematic with EMPTY lib_symbols resolves
// part types, typed pins, and net connectivity from an external .kicad_sym — including a
// derived symbol (extends) inheriting its parent's pins — and reads exactly as before
// (name-only nets, no pins) when no opener is supplied.
func TestExternalSymbolResolution(t *testing.T) {
	bare, err := ReadSchematic(bytes.NewReader(readFixture(t, "extlib.kicad_sch")), "extlib.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	if len(bare.Libraries) != 0 {
		t.Errorf("without an opener no part types resolve; got %d libraries", len(bare.Libraries))
	}
	for _, n := range bare.Nets {
		if len(n.Connections) != 0 {
			t.Errorf("without pins the SIG label names an empty net; got conns on %q: %v", n.Name, connKeys(n))
		}
	}

	d, err := ReadSchematicWithSymbols(bytes.NewReader(readFixture(t, "extlib.kicad_sch")), "extlib.kicad_sch", extOpen(t))
	if err != nil {
		t.Fatal(err)
	}
	if pt := findPartType(d, "ext:R"); pt == nil || len(pt.Pins) != 2 {
		t.Fatalf("ext:R part type missing or pinless: %+v", pt)
	}
	if pt := findPartType(d, "ext:R_Derived"); pt == nil || len(pt.Pins) != 2 {
		t.Fatalf("derived symbol must inherit the parent's pins (extends): %+v", pt)
	}
	var sig []string
	for _, n := range d.Nets {
		if n.Name == "SIG" {
			sig = connKeys(n)
		}
	}
	if strings.Join(sig, " ") != "R1.2 R2.2" {
		t.Errorf("SIG = %v, want R1.2 R2.2 (external pins drive net solving)", sig)
	}
}

// TestExternalSymbolGeometry: the same resolution feeds faithful artwork — resolved defs
// key under the QUALIFIED lib_id placements reference, not the library's bare name.
func TestExternalSymbolGeometry(t *testing.T) {
	g, err := ReadSchematicGeometryWithSymbols(bytes.NewReader(readFixture(t, "extlib.kicad_sch")), "extlib.kicad_sch", extOpen(t))
	if err != nil {
		t.Fatal(err)
	}
	refs := map[string]bool{}
	for _, sd := range g.Symbols {
		refs[sd.GetCellRef()] = true
	}
	if !refs["ext:R"] || !refs["ext:R_Derived"] {
		t.Errorf("symbol defs = %v, want qualified ext:R and ext:R_Derived", refs)
	}
}

func TestEmbeddedBeatsExternal(t *testing.T) {
	// sch.kicad_sch embeds its symbols; a poisoned opener must never be consulted for them.
	poisoned := func(lib string) ([]byte, error) {
		t.Errorf("opener consulted for embedded lib %q", lib)
		return nil, nil
	}
	if _, err := ReadSchematicWithSymbols(bytes.NewReader(readFixture(t, "sch.kicad_sch")), "sch.kicad_sch", poisoned); err != nil {
		t.Fatal(err)
	}
}

// TestExternalMatchesEmbedded is the reference comparison for WS1-016: resolving a
// library externally must yield the SAME design as embedding the same symbols in
// lib_symbols (the well-tested v6 path). kicad-cli cannot serve as the oracle here — in
// headless runs it ignores the project sym-lib-table entirely and exports an empty
// netlist — so the embedded read is the ground truth the external path is held to.
func TestExternalMatchesEmbedded(t *testing.T) {
	sch := string(readFixture(t, "extlib.kicad_sch"))
	lib := string(readFixture(t, "ext.kicad_sym"))

	// Build the embedded variant: splice the library's symbols into lib_symbols under
	// the qualified ids the placements use, with extends flattened the way KiCad saves
	// derived symbols into schematics (self-contained).
	var body strings.Builder
	for _, name := range []string{"R", "R_Derived"} {
		start := strings.Index(lib, "(symbol \""+name+"\"")
		if start < 0 {
			t.Fatalf("symbol %s not in lib", name)
		}
		depth, end := 0, -1
		for i := start; i < len(lib); i++ {
			if lib[i] == '(' {
				depth++
			}
			if lib[i] == ')' {
				depth--
				if depth == 0 {
					end = i + 1
					break
				}
			}
		}
		sym := lib[start:end]
		sym = strings.Replace(sym, "(symbol \""+name+"\"", "(symbol \"ext:"+name+"\"", 1)
		if name == "R_Derived" {
			// embedded derived symbols carry their parent's units in KiCad saves
			units := sym[:len(sym)-1] + "\n" + extractUnits(t, lib, "R") + ")"
			sym = strings.Replace(units, "(extends \"R\")", "", 1)
		}
		body.WriteString(sym + "\n")
	}
	embedded := strings.Replace(sch, "(lib_symbols)", "(lib_symbols\n"+body.String()+")", 1)

	want, err := ReadSchematic(strings.NewReader(embedded), "x.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReadSchematicWithSymbols(bytes.NewReader(readFixture(t, "extlib.kicad_sch")), "x.kicad_sch", extOpen(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Nets) != len(want.Nets) {
		t.Fatalf("nets: external %d vs embedded %d", len(got.Nets), len(want.Nets))
	}
	for i := range want.Nets {
		if want.Nets[i].Name != got.Nets[i].Name ||
			strings.Join(connKeys(want.Nets[i]), " ") != strings.Join(connKeys(got.Nets[i]), " ") {
			t.Errorf("net %d: external %s=%v vs embedded %s=%v", i,
				got.Nets[i].Name, connKeys(got.Nets[i]), want.Nets[i].Name, connKeys(want.Nets[i]))
		}
	}
	for _, name := range []string{"ext:R", "ext:R_Derived"} {
		w, g := findPartType(want, name), findPartType(got, name)
		if w == nil || g == nil || len(w.Pins) != len(g.Pins) {
			t.Errorf("%s: external %+v vs embedded %+v", name, g, w)
		}
	}
}

func extractUnits(t *testing.T, lib, name string) string {
	t.Helper()
	start := strings.Index(lib, "(symbol \""+name+"\"")
	depth, end := 0, -1
	for i := start; i < len(lib); i++ {
		if lib[i] == '(' {
			depth++
		}
		if lib[i] == ')' {
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
	}
	var units strings.Builder
	block := lib[start:end]
	for _, sub := range []string{"(symbol \"" + name + "_0_1\"", "(symbol \"" + name + "_1_1\""} {
		s2 := strings.Index(block, sub)
		if s2 < 0 {
			continue
		}
		d2, e2 := 0, -1
		for i := s2; i < len(block); i++ {
			if block[i] == '(' {
				d2++
			}
			if block[i] == ')' {
				d2--
				if d2 == 0 {
					e2 = i + 1
					break
				}
			}
		}
		units.WriteString(block[s2:e2] + "\n")
	}
	return units.String()
}
