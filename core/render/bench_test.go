package render

import "testing"

func BenchmarkSheetSVG(b *testing.B) {
	g := faithfulFixtureGeometry(b)
	sheet, err := PickSheet(g, "0")
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		_ = SheetSVG(g, sheet)
	}
}

func BenchmarkPackSheet(b *testing.B) {
	g := faithfulFixtureGeometry(b)
	sheet, err := PickSheet(g, "0")
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		_ = PackSheet(g, sheet)
	}
}
