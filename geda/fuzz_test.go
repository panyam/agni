package geda

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// seedPairs seeds the fuzzer with every (schematic, symbol) fixture pair, so both the
// schematic parser and the .sym parser start from real inputs (and CI exercises the pairs
// as ordinary test cases).
func seedPairs(f *testing.F) {
	schs, _ := filepath.Glob(filepath.Join("testdata", "*.sch"))
	syms, _ := filepath.Glob(filepath.Join("testdata", "*.sym"))
	if len(schs) == 0 || len(syms) == 0 {
		f.Fatal("no .sch/.sym fuzz seeds under testdata")
	}
	for _, sch := range schs {
		sb, err := os.ReadFile(sch)
		if err != nil {
			f.Fatal(err)
		}
		for _, sym := range syms {
			yb, err := os.ReadFile(sym)
			if err != nil {
				f.Fatal(err)
			}
			f.Add(sb, yb)
		}
	}
}

// FuzzReadWithSymbols fuzzes the schematic and symbol parsers together: the opener serves
// the fuzzed symbol bytes for every reference. Any input must error or succeed, never
// panic, and never yield a nil design without an error.
func FuzzReadWithSymbols(f *testing.F) {
	seedPairs(f)
	f.Fuzz(func(t *testing.T, sch, sym []byte) {
		open := func(string) ([]byte, error) { return sym, nil }
		d, err := ReadWithSymbols(bytes.NewReader(sch), "fuzz.sch", open)
		if err == nil && d == nil {
			t.Fatal("nil design with nil error")
		}
	})
}

// FuzzReadSchematicGeometry covers the drawing/geometry path over the same fuzzed pair.
func FuzzReadSchematicGeometry(f *testing.F) {
	seedPairs(f)
	f.Fuzz(func(t *testing.T, sch, sym []byte) {
		open := func(string) ([]byte, error) { return sym, nil }
		g, err := ReadSchematicGeometry(bytes.NewReader(sch), "fuzz.sch", open)
		if err == nil && g == nil {
			t.Fatal("nil geometry with nil error")
		}
	})
}
