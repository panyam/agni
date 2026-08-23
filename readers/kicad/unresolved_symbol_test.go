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

// TestResolvedSymbolsRecorded (agni issue 418): the references that DID load are recorded too, so
// the rule reading them can state what it examined instead of only what failed. Both routes appear
// with the kind that separates them, because only an external library can go missing on somebody
// else's machine.
//
// The PIN COUNT is the part that matters. A stale library entry resolves as successfully as the real
// symbol and answers with no pins, which costs the netlist exactly what a missing file does, and a
// record that said only "resolved" could not tell the two apart.
func TestResolvedSymbolsRecorded(t *testing.T) {
	d, err := ReadSchematicWithSymbols(bytes.NewReader(readFixture(t, "extlib.kicad_sch")), "extlib.kicad_sch", extOpen(t))
	if err != nil {
		t.Fatal(err)
	}
	diag := d.GetInputDiagnostics()
	if !slices.Contains(diag.GetSupplied(), "resolved_symbols") {
		t.Fatal("resolved_symbols not declared, so a consumer cannot tell a clean read from one nobody looked at")
	}
	got := map[string]int32{}
	for _, r := range diag.GetResolvedSymbols() {
		got[r.GetSymref()] = r.GetPinCount()
		if r.GetKind() != "kicad_sym_lib" {
			t.Errorf("%s: kind = %q, want kicad_sym_lib for a reference off the symbol path", r.GetSymref(), r.GetKind())
		}
	}
	for _, want := range []string{"ext:R", "ext:R_Derived"} {
		if n, ok := got[want]; !ok {
			t.Errorf("%s resolved but is not in the resolved set", want)
		} else if n == 0 {
			t.Errorf("%s recorded with 0 pins, so the pass carries no evidence", want)
		}
	}
}

// TestResolvedSymbolsRecordEmbedded: a schematic that carries its own symbols is the ordinary case
// and is the one that would otherwise have nothing to say. Leaving it out would mean a KiCad file
// with no external dependency produced an empty considered set, which reads as "nobody looked".
func TestResolvedSymbolsRecordEmbedded(t *testing.T) {
	d, err := ReadSchematic(bytes.NewReader(readFixture(t, "sch.kicad_sch")), "sch.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	rs := d.GetInputDiagnostics().GetResolvedSymbols()
	if len(rs) == 0 {
		t.Fatal("no resolved symbols for a schematic whose lib_symbols block defines its parts")
	}
	for _, r := range rs {
		if r.GetKind() != "kicad_sym_embedded" {
			t.Errorf("%s: kind = %q, want kicad_sym_embedded", r.GetSymref(), r.GetKind())
		}
		if r.GetPinCount() == 0 {
			t.Errorf("%s: 0 pins recorded for an embedded two-terminal part", r.GetSymref())
		}
	}
}

// TestResolvedSymbolsExcludeFailures: a reference that did not open must not appear on both lists.
// The two are one partition, and a reference in both would let a rule count the same subject twice
// and report it as passed and failed at once.
func TestResolvedSymbolsExcludeFailures(t *testing.T) {
	failing := func(lib string) ([]byte, error) { return nil, fmt.Errorf("no library %q", lib) }
	d, err := ReadSchematicWithSymbols(bytes.NewReader(readFixture(t, "extlib.kicad_sch")), "extlib.kicad_sch", failing)
	if err != nil {
		t.Fatal(err)
	}
	diag := d.GetInputDiagnostics()
	resolved := map[string]bool{}
	for _, r := range diag.GetResolvedSymbols() {
		resolved[r.GetSymref()] = true
	}
	for _, u := range diag.GetUnresolvedSymbols() {
		if resolved[u.GetSymref()] {
			t.Errorf("%s appears as both resolved and unresolved", u.GetSymref())
		}
	}
}
