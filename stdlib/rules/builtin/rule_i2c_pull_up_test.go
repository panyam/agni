package builtin

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestIsI2CBoundary (WS3-037): SDA/SCL match at a token boundary, not as a substring. The load-bearing
// case is SPI_SCLK — an SPI clock, not an I2C net — which the old strings.Contains match wrongly caught.
func TestIsI2CBoundary(t *testing.T) {
	cases := map[string]bool{
		"SDA": true, "SCL": true, "I2C_SCL": true, "SCL0": true, "SDA_1": true, "SENSOR_SDA": true,
		"sda":      true, // case-folded
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

// comps builds the component list for a topology test. Class comes from the ref-des prefix, so
// naming them R* / U* is what makes them resistors and parts.
func comps(refs ...string) []*ir.Component {
	out := make([]*ir.Component, 0, len(refs))
	for _, r := range refs {
		out = append(out, &ir.Component{RefDes: r, Prov: &ir.Provenance{SourceFile: "t"}})
	}
	return out
}

// TestI2CPullUpTopologies is the acceptance for agni issue 375. The rule used to ask whether ANY
// resistor touched the bus, so every row below where a resistor exists but reaches no rail passed an
// error-severity check while the bus was electrically unheld.
//
// The extended-net rows are the reason this is a WALK and not a one-hop test. A bus segment
// separated from its pull-up by a series isolation or termination resistor is an ordinary topology,
// and a one-hop rule would have fired on it: a false positive traded for the false pass, which is no
// improvement on a rule at error severity.
func TestI2CPullUpTopologies(t *testing.T) {
	for _, tc := range []struct {
		name      string
		comps     []string
		nets      []*ir.Net
		wantFires bool
		why       string
	}{
		{
			name:  "direct pull-up to a rail",
			comps: []string{"U1", "R1"},
			nets: []*ir.Net{
				tnet("SCL", "U1.1", "R1.1"),
				tnet("+3V3", "R1.2"),
			},
			wantFires: false,
			why:       "the ordinary case; if this fires the rule is useless",
		},
		{
			name:  "series resistor to another signal, no pull-up anywhere",
			comps: []string{"U1", "U2", "R1"},
			nets: []*ir.Net{
				tnet("SCL", "U1.1", "R1.1"),
				tnet("SCL_ISO", "R1.2", "U2.1"),
			},
			wantFires: true,
			why:       "THE BUG: a series resistor is not a pull-up, and this passed before issue 375",
		},
		{
			name:  "resistor to ground is a pull-down",
			comps: []string{"U1", "R1"},
			nets: []*ir.Net{
				tnet("SCL", "U1.1", "R1.1"),
				tnet("GND", "R1.2"),
			},
			wantFires: true,
			why:       "a pull-down holds the bus low, which is the failure this rule reports",
		},
		{
			name:      "no resistor at all",
			comps:     []string{"U1"},
			nets:      []*ir.Net{tnet("SCL", "U1.1")},
			wantFires: true,
			why:       "the case the rule always caught; must keep catching it",
		},
		{
			name:  "extended net: pull-up beyond one series resistor",
			comps: []string{"U1", "U2", "R1", "R2"},
			nets: []*ir.Net{
				tnet("SCL", "U1.1", "R1.1"),
				tnet("SCL_ISO", "R1.2", "U2.1", "R2.1"),
				tnet("+3V3", "R2.2"),
			},
			wantFires: false,
			why:       "the bus IS held high, one hop further out; a one-hop rule would false-positive here",
		},
		{
			name:  "extended net: pull-up beyond two series resistors",
			comps: []string{"U1", "R1", "R2", "R3"},
			nets: []*ir.Net{
				tnet("SCL", "U1.1", "R1.1"),
				tnet("SEG1", "R1.2", "R2.1"),
				tnet("SEG2", "R2.2", "R3.1"),
				tnet("+3V3", "R3.2"),
			},
			wantFires: false,
			why:       "three crossings, the bound; still held, still unusual",
		},
		{
			name:  "beyond the bound: pull-up four crossings away",
			comps: []string{"U1", "R1", "R2", "R3", "R4"},
			nets: []*ir.Net{
				tnet("SCL", "U1.1", "R1.1"),
				tnet("SEG1", "R1.2", "R2.1"),
				tnet("SEG2", "R2.2", "R3.1"),
				tnet("SEG3", "R3.2", "R4.1"),
				tnet("+3V3", "R4.2"),
			},
			wantFires: true,
			why:       "pins PullUpReachHops: four series resistors is not a pull-up on this bus",
		},
		{
			name:  "pull-down present AND a real pull-up",
			comps: []string{"U1", "R1", "R2"},
			nets: []*ir.Net{
				tnet("SCL", "U1.1", "R1.1", "R2.1"),
				tnet("GND", "R1.2"),
				tnet("+3V3", "R2.2"),
			},
			wantFires: false,
			why:       "ground must not be traversed, and must not mask a rail reachable another way",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &ir.Design{Components: comps(tc.comps...), Nets: tc.nets}
			got := firedSubjects(d, "i2c-pull-up")["SCL"]
			if got != tc.wantFires {
				t.Errorf("fires = %v, want %v (%s)", got, tc.wantFires, tc.why)
			}
		})
	}
}
