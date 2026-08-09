package xschem

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

func readSch(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestUnresolvedSymbolReported (WS1-052): an opener that cannot find a .sym leaves every placement
// pinless, and the reference is now recorded instead of only making the read quieter.
func TestUnresolvedSymbolReported(t *testing.T) {
	failing := func(symref string) ([]byte, error) { return nil, fmt.Errorf("no symbol %q", symref) }
	d, err := ReadWithSymbols(bytes.NewReader(readSch(t, "divider.sch")), "divider.sch", failing)
	if err != nil {
		t.Fatal(err)
	}
	us := d.GetInputDiagnostics().GetUnresolvedSymbols()
	if len(us) == 0 {
		t.Fatal("no unresolved symbols reported for a read where every symbol failed to open")
	}
	for _, u := range us {
		if u.GetKind() != "xschem_sym" {
			t.Errorf("%s: kind = %q, want xschem_sym", u.GetSymref(), u.GetKind())
		}
		if len(u.GetRefDes()) == 0 {
			t.Errorf("%s: no ref_des recorded", u.GetSymref())
		}
	}
}

// TestUnresolvedSymbolSuppressesDanglesAndSaysWhy is the pairing that motivates the whole ticket:
// the reader already suppressed dangling endpoints when a symbol failed (WS1-013), because missing
// pins turn real wire ends into phantom dangles. Before this, that suppression was the ONLY effect
// and it was invisible. Both must now hold at once — dangles quiet, cause reported.
func TestUnresolvedSymbolSuppressesDanglesAndSaysWhy(t *testing.T) {
	failing := func(string) ([]byte, error) { return nil, fmt.Errorf("missing") }
	d, err := ReadWithSymbols(bytes.NewReader(readSch(t, "dangle.sch")), "dangle.sch", failing)
	if err != nil {
		t.Fatal(err)
	}
	diag := d.GetInputDiagnostics()
	if len(diag.GetDanglingEndpoints()) != 0 {
		t.Errorf("dangling endpoints = %d, want 0 (suppressed while pins are unknown)", len(diag.GetDanglingEndpoints()))
	}
	if len(diag.GetUnresolvedSymbols()) == 0 {
		t.Error("dangles suppressed with nothing recorded to explain why; that is the silence WS1-052 closes")
	}
}

// TestResolvedSymbolReportsNothing: the fixture's own symbols resolve, so a clean read stays clean.
// Guards against a reader that flags unconditionally, which would make the diagnostic worthless.
func TestResolvedSymbolReportsNothing(t *testing.T) {
	open := func(symref string) ([]byte, error) { return os.ReadFile("testdata/" + symref) }
	d, err := ReadWithSymbols(bytes.NewReader(readSch(t, "divider.sch")), "divider.sch", open)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range d.GetInputDiagnostics().GetUnresolvedSymbols() {
		if u.GetSymref() == "res.sym" {
			t.Errorf("res.sym reported unresolved, but testdata/res.sym exists and opened")
		}
	}
}
