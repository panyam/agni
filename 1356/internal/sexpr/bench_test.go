package sexpr

import (
	"bytes"
	"os"
	"testing"
)

// BenchmarkParse parses a committed KiCad board, a committed EDIF schematic export, and the largest
// fetched sample board, an 80 MB .kicad_pcb, when tools/samples holds it (make samples-oracle).
func BenchmarkParse(b *testing.B) {
	cases := []struct {
		name, path string
		mode       StringMode
	}{
		{"kicad-fixture", "../../readers/kicad/testdata/board.kicad_pcb", KiCadStrings},
		{"edif-fixture", "../../readers/edif/testdata/hidden-field-flood.eds", EDIFStrings},
		{"kicad-80MB", "../../tools/samples/boards/jetson-agx-thor-baseboard/jetson-agx-thor-baseboard.kicad_pcb", KiCadStrings},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			data, err := os.ReadFile(c.path)
			if os.IsNotExist(err) {
				b.Skipf("%s is absent", c.path)
			}
			if err != nil {
				b.Fatal(err)
			}
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			for b.Loop() {
				if _, err := Parse(bytes.NewReader(data), c.mode); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
