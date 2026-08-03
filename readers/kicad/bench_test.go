package kicad

import (
	"bytes"
	"os"
	"testing"
)

func BenchmarkReadSchematic(b *testing.B) {
	data, err := os.ReadFile("testdata/sch.kicad_sch")
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		if _, err := ReadSchematic(bytes.NewReader(data), "bench.kicad_sch"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRead(b *testing.B) {
	data, err := os.ReadFile("testdata/pcb.kicad_pcb")
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		if _, err := Read(bytes.NewReader(data), "bench.kicad_pcb"); err != nil {
			b.Fatal(err)
		}
	}
}
