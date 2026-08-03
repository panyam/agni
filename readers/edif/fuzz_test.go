package edif

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

// FuzzRead throws arbitrary bytes at the EDIF netlist parser: it must error, never panic,
// and never return a nil design without an error.
func FuzzRead(f *testing.F) {
	seedCorpus(f, "*.edn")
	f.Fuzz(func(t *testing.T, data []byte) {
		d, err := Read(bytes.NewReader(data), "fuzz.edn")
		if err == nil && d == nil {
			t.Fatal("nil design with nil error")
		}
	})
}

// FuzzReadSchematic covers the .eds geometry half of the parser the same way.
func FuzzReadSchematic(f *testing.F) {
	seedCorpus(f, "*.eds")
	f.Fuzz(func(t *testing.T, data []byte) {
		g, err := ReadSchematic(bytes.NewReader(data), "fuzz.eds")
		if err == nil && g == nil {
			t.Fatal("nil geometry with nil error")
		}
	})
}
