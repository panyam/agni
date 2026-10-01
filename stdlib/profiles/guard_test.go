package profiles

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/query"
)

// TestBuiltinsCompileAsInjective is the WS3-127 regression guard. Every requirement compiler routes
// its query through mustBeInjective, which panics on a rule whose body puts two variables in one
// argument position with nothing separating them. Compiling every built-in profile is the assertion, so
// a future requirement type added to any of these fails here rather than on a customer's board.
// WS3-114's shape, a rule opening with an unbound walk, is covered where it is now prevented, by
// TestTheDefaultEvaluatorPlansAnUnboundWalk in core/query.
//
// A wall-clock assertion would be the obvious test and is the wrong one. It is flaky on CI and it
// only fires on a design big enough to hurt, which no fixture in this repo is. That is why
// the original regression shipped green.
func TestBuiltinsCompileAsInjective(t *testing.T) {
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

// The gate has to REJECT, not merely exist. Compiling the built-ins proves nothing about the check
// if the check never fires, which is the failure mode a lint is most prone to, passing forever
// because it cannot see anything.
//
// The shape here is the interface-presence rule with its disequality removed, which is the real bug
// this guards. One matching signal would satisfy both has_signal atoms, so the interface reports
// itself in use on half the evidence.
func TestGateRejectsANonInjectiveRule(t *testing.T) {
	bad := query.MustParse(
		`has_signal(?x) :- component.net(?x,?n); in_use(?x) :- has_signal(?x), has_signal(?y); in_use(?z) => ?z`)
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("mustBeInjective accepted a rule where one node satisfies two signal atoms")
		}
		if msg, ok := r.(string); !ok || !strings.Contains(msg, "in_use") {
			t.Errorf("panic should name the offending head relation, got %v", r)
		}
	}()
	mustBeInjective(bad)
}
