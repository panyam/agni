package kicad

import (
	"bytes"
	"fmt"
	"slices"
	"testing"
)

// TestUnresolvedSymbolReported (WS1-052): a lib_id the schematic does not embed AND whose external
// library fails to open is recorded, with every placement it cost pins. extlib.kicad_sch has an
// empty (lib_symbols) block, so both its parts depend entirely on the external "ext" library.
func TestUnresolvedSymbolReported(t *testing.T) {
	failing := func(lib string) ([]byte, error) { return nil, fmt.Errorf("no library %q", lib) }
	d, err := ReadSchematicWithSymbols(bytes.NewReader(readFixture(t, "extlib.kicad_sch")), "extlib.kicad_sch", failing)
	if err != nil {
		t.Fatal(err)
	}
	us := d.GetInputDiagnostics().GetUnresolvedSymbols()
	if len(us) != 2 {
		t.Fatalf("unresolved symbols = %d (%v), want 2 (ext:R and ext:R_Derived)", len(us), us)
	}
	var refs []string
	for _, u := range us {
		if u.GetKind() != "kicad_sym_lib" {
			t.Errorf("%s: kind = %q, want kicad_sym_lib", u.GetSymref(), u.GetKind())
		}
		if u.GetProv().GetSourceFile() != "extlib.kicad_sch" {
			t.Errorf("%s: prov source = %q, want the schematic", u.GetSymref(), u.GetProv().GetSourceFile())
		}
		refs = append(refs, u.GetRefDes()...)
	}
	if len(refs) == 0 {
		t.Error("no ref_des recorded; the finding cannot say what the missing library cost")
	}
}

// TestUnresolvedSymbolSilentWhenResolved: the real library resolves both lib_ids, so nothing is
// reported. Without this the test above would pass on a reader that flagged unconditionally.
func TestUnresolvedSymbolSilentWhenResolved(t *testing.T) {
	d, err := ReadSchematicWithSymbols(bytes.NewReader(readFixture(t, "extlib.kicad_sch")), "extlib.kicad_sch", extOpen(t))
	if err != nil {
		t.Fatal(err)
	}
	if us := d.GetInputDiagnostics().GetUnresolvedSymbols(); len(us) != 0 {
		t.Errorf("unresolved symbols = %v, want none when the library resolves", us)
	}
}

// TestUnresolvedSymbolSilentWithoutOpener: reading with no opener at all is a caller deliberately
// asking for a symbol-free read (the plain ReadSchematic entry), not a resolution failure. Flagging
// it would fire on every such read and train the reader to ignore the diagnostic.
func TestUnresolvedSymbolSilentWithoutOpener(t *testing.T) {
	d, err := ReadSchematic(bytes.NewReader(readFixture(t, "extlib.kicad_sch")), "extlib.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	if us := d.GetInputDiagnostics().GetUnresolvedSymbols(); len(us) != 0 {
		t.Errorf("unresolved symbols = %v, want none for a no-opener read", us)
	}
}

// TestUnresolvedSymbolGroupsPlacements: one missing library that several parts share is ONE record
// carrying both designators, not one record per part. The grouping is what keeps a single missing
// file from reading as N separate problems.
func TestUnresolvedSymbolGroupsPlacements(t *testing.T) {
	failing := func(string) ([]byte, error) { return nil, fmt.Errorf("missing") }
	d, err := ReadSchematicWithSymbols(bytes.NewReader(readFixture(t, "extlib.kicad_sch")), "extlib.kicad_sch", failing)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range d.GetInputDiagnostics().GetUnresolvedSymbols() {
		if slices.Contains(u.GetRefDes(), "") {
			t.Errorf("%s: empty ref_des recorded", u.GetSymref())
		}
	}
}
