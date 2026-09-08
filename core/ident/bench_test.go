package ident

import "testing"

var sink Result
var ssink string

func BenchmarkCanonicalPlain(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ssink = Canonical("GMAC0_MII_RGMII_TXD1")
	}
}

func BenchmarkCompareExact(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sink = Compare("PTC11", "PTC11")
	}
}

func BenchmarkCompareNormalized(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sink = Compare("PTE7", "PTE07")
	}
}

func BenchmarkCompareNoMatch(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sink = Compare("GMAC0_MII_RGMII_TXD1", "GMAC0_MII_RGMII_TXD2")
	}
}

// The shape a real map hits: a multi-valued cell on both sides, which is where the nested
// alternative loops do the most work.
func BenchmarkCompareMultiValuedNoMatch(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sink = Compare("GPIO[34] / WKPU[8] / ADC0_S17", "GPIO[35] / WKPU[9] / ADC0_S18")
	}
}
