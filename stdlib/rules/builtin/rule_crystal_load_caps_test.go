package builtin

import (
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// crystalFixture exercises the guard matrix in one design:
//   - Y1 passive crystal, one terminal (XOUT1) has no load cap -> fires (subject Y1).
//   - Y3 passive crystal, both terminals have a cap -> silent.
//   - Y2 active oscillator (a non-ground power rail on a pin) -> skipped even with no caps.
//   - Y4 active oscillator whose Vcc net reads as neither a rail nor ground (the real EDIF
//     corpus case: an OUTPUT/Vcc/Standby oscillator, 3 non-ground terminals) -> skipped by the
//     "exactly two terminals" gate, so no false positive even without a recognizable rail.
//   - Y5 passive crystal whose cap-less terminal is an unresolved external net -> skipped.
//   - U9 is not a crystal, so its cap-less net is never a subject.
func crystalFixture() *ir.Design {
	comp := func(ref string) *ir.Component {
		return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"}}
	}
	oscVdd := tnet("OSC_VDD", "Y2.1")
	oscVdd.Attributes = map[string]string{netgraph.AttrPowerDriven: "true"}
	xext := tnet("XEXT5", "Y5.1")
	xext.Attributes = map[string]string{netgraph.AttrExternal: "true"}
	return &ir.Design{
		Components: []*ir.Component{
			comp("U1"), comp("U9"),
			comp("Y1"), comp("Y2"), comp("Y3"), comp("Y4"), comp("Y5"),
			comp("C1"), comp("C3"), comp("C4"), comp("C5"),
		},
		Nets: []*ir.Net{
			tnet("XIN1", "Y1.1", "U1.1", "C1.1"),
			tnet("XOUT1", "Y1.2", "U1.2"),
			tnet("XIN3", "Y3.1", "C3.1"),
			tnet("XOUT3", "Y3.2", "C4.1"),
			oscVdd,
			tnet("OSC_OUT", "Y2.2"),
			// Y4: OUTPUT/Vcc/Standby active oscillator; the Vcc net "$2N999" is not a
			// recognized rail, so only the 3-non-ground-terminal gate excludes it.
			tnet("OSC_OUT4", "Y4.3"),
			tnet("$2N999", "Y4.4"),
			tnet("STBY4", "Y4.1"),
			tnet("SIG", "U9.1"),
			xext,
			tnet("XIN5", "Y5.2", "C5.1"),
		},
	}
}

func TestCrystalLoadCaps(t *testing.T) {
	fs := crystalLoadCaps.Eval(check.NewModel(crystalFixture()))
	if len(fs) != 1 || fs[0].Subject != "Y1" || fs[0].Kind != check.KindComponent {
		t.Fatalf("findings = %+v, want exactly one KindComponent finding on Y1", fs)
	}
	if fs[0].Message != "crystal terminal net XOUT1 has no load capacitor" {
		t.Errorf("message = %q", fs[0].Message)
	}
}
