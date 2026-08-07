package profiles

import (
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// netNames is the shape Diagnose reads: just the design's net names.
func netNames(d *ir.Design) []string {
	out := make([]string, 0, len(d.Nets))
	for _, n := range d.Nets {
		out = append(out, n.GetName())
	}
	return out
}

// manyNets builds n nets named <prefix>0.._H style, for exercising the share threshold.
func manyNets(n int, suffix string) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, "NET"+string(rune('A'+i%26))+string(rune('0'+i/26))+suffix)
	}
	return out
}

// The motivating case (WS3-101): an unanchored regex that is a legitimate pattern but claims every
// _H net on the board. Nothing at load time can catch it, because the verdict needs the design.
func TestDiagnoseFlagsOverBroadMatcher(t *testing.T) {
	p := Profile{
		Name:    "MYBUS",
		Signals: []Signal{{Name: "H", Regex: "_H", Anchor: true}, {Name: "L", Suffix: "_MYBUS_L"}},
	}
	msgs := Diagnose(manyNets(40, "_H"), p)
	if len(msgs) == 0 {
		t.Fatal("want a diagnostic for a matcher claiming every net")
	}
	joined := strings.Join(msgs, "\n")
	for _, want := range []string{"MYBUS", `"H"`, "_H", "40 of 40"} {
		if !strings.Contains(joined, want) {
			t.Errorf("diagnostic should mention %q, got: %s", want, joined)
		}
	}
}

// Two roles of one profile resolving to the same net is broken with no threshold to argue about:
// whichever generated rule runs first claims it.
func TestDiagnoseFlagsCollidingSignals(t *testing.T) {
	p := Profile{
		Name: "MYBUS",
		Signals: []Signal{
			{Name: "TX", Suffix: "_TX", Anchor: true},
			{Name: "DATA", Suffix: "X"}, // also matches every _TX net
		},
	}
	msgs := Diagnose([]string{"BUS_TX", "BUS_RX", "OTHER"}, p)
	joined := strings.Join(msgs, "\n")
	if !strings.Contains(joined, `"TX"`) || !strings.Contains(joined, `"DATA"`) || !strings.Contains(joined, "BUS_TX") {
		t.Errorf("want a collision naming both roles and the net, got: %s", joined)
	}
}

// A design too small for a share to mean anything stays quiet: on four nets, one match is 25% and
// says nothing about the matcher. Without the floor every correct profile on a fixture would warn.
func TestDiagnoseQuietBelowFloor(t *testing.T) {
	p := Profile{Name: "MYBUS", Signals: []Signal{{Name: "H", Suffix: "_H", Anchor: true}}}
	if msgs := Diagnose([]string{"A_H", "B", "C", "D"}, p); len(msgs) != 0 {
		t.Errorf("want silence below the floor, got: %v", msgs)
	}
}

// Multi-instance naming is CORRECT, not over-broad: a role legitimately matches one net per channel.
// This is the false positive the share threshold is set to avoid — 16 LIN channels on a 200-net
// board is 8%, well under the bar.
func TestDiagnoseQuietOnMultiInstanceNaming(t *testing.T) {
	p := Profile{Name: "LIN_MULTI", Signals: []Signal{{Name: "TX", Suffix: "_TX", Anchor: true}}}
	nets := make([]string, 0, 200)
	for i := 0; i < 16; i++ {
		nets = append(nets, "LIN_"+string(rune('A'+i))+"_TX")
	}
	for i := len(nets); i < 200; i++ {
		nets = append(nets, "UNRELATED_"+string(rune('A'+i%26))+string(rune('0'+i/26)))
	}
	if msgs := Diagnose(nets, p); len(msgs) != 0 {
		t.Errorf("16 channels on a 200-net board is correct multi-instance naming, got: %v", msgs)
	}
}

// The shipped built-ins must stay silent on the designs their own behavioral suites use, or the
// warning is noise from the first run. This is the gate the ticket names.
func TestDiagnoseQuietOnBuiltins(t *testing.T) {
	designs := map[string]*ir.Design{
		"spinorGood":   spinorGood(),
		"spinorBroken": spinorBroken(),
	}
	for dn, d := range designs {
		for _, p := range Profiles {
			if msgs := Diagnose(netNames(d), p); len(msgs) != 0 {
				t.Errorf("built-in %q on %s should be silent, got: %v", p.Name, dn, msgs)
			}
		}
	}
}

// A profile with no design to judge against, and a design with no nets, both produce nothing rather
// than dividing by zero.
func TestDiagnoseEmptyInputs(t *testing.T) {
	if msgs := Diagnose(nil, CAN); len(msgs) != 0 {
		t.Errorf("no nets should yield no diagnostics, got: %v", msgs)
	}
	if msgs := Diagnose([]string{"A", "B"}, Profile{Name: "EMPTY"}); len(msgs) != 0 {
		t.Errorf("no signals should yield no diagnostics, got: %v", msgs)
	}
}
