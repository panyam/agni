package check

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestIsI2CBoundary (WS3-037): SDA/SCL match at a token boundary, not as a substring. The load-bearing
// case is SPI_SCLK — an SPI clock, not an I2C net — which the old strings.Contains match wrongly caught.
func TestIsI2CBoundary(t *testing.T) {
	cases := map[string]bool{
		"SDA": true, "SCL": true, "I2C_SCL": true, "SCL0": true, "SDA_1": true, "SENSOR_SDA": true,
		"sda": true, // case-folded
		"SPI_SCLK": false, "SCLK": false, "MCLK": false, "MISCL": false, "OSCL": false, "SCLK0": false,
	}
	for name, want := range cases {
		if got := isI2C(name); got != want {
			t.Errorf("isI2C(%q) = %v, want %v", name, got, want)
		}
	}
}

// TestI2CPullUpSCLKNotFlagged (WS3-037): the rule fires on a bare SCL with no pull-up but NOT on an
// SPI_SCLK net (the false positive) — an error-severity finding on every SPI clock line before the fix.
func TestI2CPullUpSCLKNotFlagged(t *testing.T) {
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "t"}}},
		Nets: []*ir.Net{
			{Name: "SPI_SCLK", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}},
			{Name: "SCL", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "2"}}},
		},
	}
	got := firedSubjects(d, "i2c-pull-up")
	if got["SPI_SCLK"] {
		t.Error("SPI_SCLK is an SPI clock, not I2C — must not be flagged by i2c-pull-up")
	}
	if !got["SCL"] {
		t.Error("SCL with no pull-up should be flagged")
	}
}
