package edif

import (
	"bytes"
	"os"
	"testing"
)

// Benchmarks baseline the parse cost per fixture byte (C7 calls large-file parsing a
// server-side concern; these are the tripwire, sized-up corpus runs are manual).
func BenchmarkRead(b *testing.B) {
	data, err := os.ReadFile("testdata/basic.edn")
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		if _, err := Read(bytes.NewReader(data), "bench.edn"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadSchematic(b *testing.B) {
	data, err := os.ReadFile("testdata/sample.eds")
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		if _, err := ReadSchematic(bytes.NewReader(data), "bench.eds"); err != nil {
			b.Fatal(err)
		}
	}
}
