package geda

import (
	"bytes"
	"fmt"
	"os"
	"slices"
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

// TestResolvedSymbolsRecorded (agni issue 418): the references that loaded are recorded beside the
// ones that did not, with the pin count each supplied. Without this a rule over symbol resolution
// holds only a failure list and can therefore report only failures, so a design where everything
// loaded looks exactly like one the reader never opened a symbol for.
//
// The COUNT is what makes it evidence. A stale library answering with an empty stub resolves as
// successfully as the real symbol and costs the netlist the same pins.
func TestResolvedSymbolsRecorded(t *testing.T) {
	d, err := ReadWithSymbols(bytes.NewReader(readGedaSch(t, "divider.sch")), "divider.sch", func(symref string) ([]byte, error) { return os.ReadFile("testdata/" + symref) })
	if err != nil {
		t.Fatal(err)
	}
	diag := d.GetInputDiagnostics()
	if !slices.Contains(diag.GetSupplied(), "resolved_symbols") {
		t.Fatal("resolved_symbols not declared, so a consumer cannot tell a clean read from one nobody looked at")
	}
	rs := diag.GetResolvedSymbols()
	if len(rs) == 0 {
		t.Fatal("no resolved symbols recorded for a read where the library opened")
	}
	for _, r := range rs {
		if r.GetKind() != "geda_sym" {
			t.Errorf("%s: kind = %q, want geda_sym", r.GetSymref(), r.GetKind())
		}
		if r.GetPinCount() == 0 {
			t.Errorf("%s: 0 pins recorded, so the pass carries no evidence", r.GetSymref())
		}
	}
}

// TestResolvedSymbolsAbsentWithoutOpener is the honesty half. Reading with no opener is a caller
// deliberately asking for a symbol-free read, and declaring the diagnostic there would turn "we did
// not look" into "we looked and found nothing missing", which is the coverage claim the whole
// considered-set idea exists to withhold.
func TestResolvedSymbolsAbsentWithoutOpener(t *testing.T) {
	d, err := Read(bytes.NewReader(readGedaSch(t, "divider.sch")), "divider.sch")
	if err != nil {
		t.Fatal(err)
	}
	diag := d.GetInputDiagnostics()
	if slices.Contains(diag.GetSupplied(), "resolved_symbols") {
		t.Error("resolved_symbols declared on a read that opened no symbol at all")
	}
	if rs := diag.GetResolvedSymbols(); len(rs) != 0 {
		t.Errorf("resolved symbols = %v, want none for a no-opener read", rs)
	}
}

// TestResolvedSymbolsExcludeFailures: the two lists are one partition, and a reference on both would
// let a rule report the same subject as passed and failed at once.
func TestResolvedSymbolsExcludeFailures(t *testing.T) {
	failing := func(string) ([]byte, error) { return nil, fmt.Errorf("missing") }
	d, err := ReadWithSymbols(bytes.NewReader(readGedaSch(t, "divider.sch")), "divider.sch", failing)
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
