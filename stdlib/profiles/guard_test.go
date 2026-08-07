package profiles

import "testing"

// TestBuiltinsCompileWithoutGeneratorFirst is the WS3-114 regression guard. Every requirement
// compiler routes its query through mustBindHeadFirst, which panics on a rule that opens with an
// unbound reaches, so compiling every built-in profile is the assertion: a future requirement type
// added to any of these fails here rather than on a customer's board.
//
// A wall-clock assertion would be the obvious test and is the wrong one — it is flaky on CI and it
// only fires on a design big enough to hurt, which no fixture in this repo is. That is exactly why
// the original regression shipped green.
func TestBuiltinsCompileWithoutGeneratorFirst(t *testing.T) {
	for _, c := range []struct {
		name string
		p    Profile
	}{
		{"SPI_NOR", SPINOR}, {"eMMC", EMMC}, {"CAN", CAN}, {"LIN", LIN},
		{"A2B", A2B}, {"PCIe", PCIE}, {"SGMII", SGMII},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := len(Compile(c.p)); got == 0 {
				t.Fatalf("%s compiled no rules, so this test would pass vacuously", c.name)
			}
		})
	}
}
