package kicad

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// seedCorpus adds every matching testdata fixture as a fuzz seed, so the fuzzer starts
// from real, structurally valid inputs (and CI exercises them as ordinary test cases).
func seedCorpus(f *testing.F, glob string) {
	seeds, err := filepath.Glob(filepath.Join("testdata", glob))
	if err != nil || len(seeds) == 0 {
		f.Fatalf("no fuzz seeds for %q under testdata", glob)
	}
	for _, s := range seeds {
		b, err := os.ReadFile(s)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
	}
}

// FuzzRead throws arbitrary bytes at the board (s-expr) parser: it must error, never
// panic, and never return a nil design without an error.
func FuzzRead(f *testing.F) {
	seedCorpus(f, "*.kicad_pcb")
	f.Fuzz(func(t *testing.T, data []byte) {
		d, err := Read(bytes.NewReader(data), "fuzz.kicad_pcb")
		if err == nil && d == nil {
			t.Fatal("nil design with nil error")
		}
	})
}

// FuzzReadSchematic covers the schematic netlist path over the same s-expr tokenizer.
func FuzzReadSchematic(f *testing.F) {
	seedCorpus(f, "*.kicad_sch")
	f.Fuzz(func(t *testing.T, data []byte) {
		d, err := ReadSchematic(bytes.NewReader(data), "fuzz.kicad_sch")
		if err == nil && d == nil {
			t.Fatal("nil design with nil error")
		}
	})
}

// FuzzReadSchematicGeometry covers the geometry extraction, which walks the same tree with
// its own placement/shape logic.
func FuzzReadSchematicGeometry(f *testing.F) {
	seedCorpus(f, "*.kicad_sch")
	f.Fuzz(func(t *testing.T, data []byte) {
		g, err := ReadSchematicGeometry(bytes.NewReader(data), "fuzz.kicad_sch")
		if err == nil && g == nil {
			t.Fatal("nil geometry with nil error")
		}
	})
}
