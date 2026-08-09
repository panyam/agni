package geda

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

func readGedaSch(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestUnresolvedSymbolReported (WS1-052): a failing opener leaves every placement pinless, and the
// reference is recorded rather than only suppressing the dangle set.
func TestUnresolvedSymbolReported(t *testing.T) {
	failing := func(symref string) ([]byte, error) { return nil, fmt.Errorf("no symbol %q", symref) }
	d, err := ReadWithSymbols(bytes.NewReader(readGedaSch(t, "divider.sch")), "divider.sch", failing)
	if err != nil {
		t.Fatal(err)
	}
	us := d.GetInputDiagnostics().GetUnresolvedSymbols()
	if len(us) == 0 {
		t.Fatal("no unresolved symbols reported for a read where every symbol failed to open")
	}
	for _, u := range us {
		if u.GetKind() != "geda_sym" {
			t.Errorf("%s: kind = %q, want geda_sym", u.GetSymref(), u.GetKind())
		}
		if len(u.GetRefDes()) == 0 {
			t.Errorf("%s: no ref_des recorded", u.GetSymref())
		}
		if u.GetProv().GetSourceFile() != "divider.sch" {
			t.Errorf("%s: prov source = %q, want the schematic", u.GetSymref(), u.GetProv().GetSourceFile())
		}
	}
}

// TestUnresolvedSymbolSuppressesDanglesAndSaysWhy: dangles stay suppressed (WS1-013) AND the cause
// is now recorded. The suppression alone was the invisible failure this ticket closes.
func TestUnresolvedSymbolSuppressesDanglesAndSaysWhy(t *testing.T) {
	failing := func(string) ([]byte, error) { return nil, fmt.Errorf("missing") }
	d, err := ReadWithSymbols(bytes.NewReader(readGedaSch(t, "dangle.sch")), "dangle.sch", failing)
	if err != nil {
		t.Fatal(err)
	}
	diag := d.GetInputDiagnostics()
	if len(diag.GetDanglingEndpoints()) != 0 {
		t.Errorf("dangling endpoints = %d, want 0 (suppressed while pins are unknown)", len(diag.GetDanglingEndpoints()))
	}
	if len(diag.GetUnresolvedSymbols()) == 0 {
		t.Error("dangles suppressed with nothing recorded to explain why")
	}
}

// TestResolvedSymbolReportsNothing: divider.sch's symbols live beside it, so a real opener yields a
// clean read. Guards against an unconditional flag.
func TestResolvedSymbolReportsNothing(t *testing.T) {
	open := func(symref string) ([]byte, error) { return os.ReadFile("testdata/" + symref) }
	d, err := ReadWithSymbols(bytes.NewReader(readGedaSch(t, "divider.sch")), "divider.sch", open)
	if err != nil {
		t.Fatal(err)
	}
	if us := d.GetInputDiagnostics().GetUnresolvedSymbols(); len(us) != 0 {
		t.Errorf("unresolved = %v, want none when every symbol opens", us)
	}
}
