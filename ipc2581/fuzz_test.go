package ipc2581

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

// FuzzRead covers the hand-rolled post-decode mapping behind the XML decode: arbitrary
// bytes must error, never panic, and never yield a nil design without an error.
func FuzzRead(f *testing.F) {
	seedCorpus(f, "*.xml")
	f.Fuzz(func(t *testing.T, data []byte) {
		d, err := Read(bytes.NewReader(data), "fuzz.xml")
		if err == nil && d == nil {
			t.Fatal("nil design with nil error")
		}
	})
}
