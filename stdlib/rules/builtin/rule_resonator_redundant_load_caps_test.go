package builtin

import (
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// resonatorFixture exercises the guard matrix in one design:
//   - Y1 ceramic resonator, terminal XIN carries an external load cap C1 to ground -> fires (Y1, XIN).
//   - Y1's other terminal XOUT has no external cap -> not a second finding.
//   - Y2 ceramic resonator wired with no external caps (only its built-in caps) -> silent.
//   - Y3 CRYSTAL with a load cap to ground -> silent: a crystal needs its caps (crystal-load-caps'
//     job), so the resonator rule must not fire on it.
//   - Y4 ceramic resonator whose terminal net XEXT is an unresolved external net -> skipped.
//   - C5 is a COUPLING cap between two signal nets (never touches ground) -> not a load cap, so a
//     resonator terminal carrying only C5 is not flagged.
func resonatorFixture() *ir.Design {
	comp := func(ref string, classes ...string) *ir.Component {
		return &ir.Component{RefDes: ref, DeviceClasses: classes, Prov: &ir.Provenance{SourceFile: "t"}}
	}
	reso := func(ref string) *ir.Component { return comp(ref, "ceramic_resonator", "clock") }
	xext := tnet("XEXT", "Y4.1", "C4.1")
	xext.Attributes = map[string]string{netgraph.AttrExternal: "true"}
	return &ir.Design{
		Components: []*ir.Component{
			comp("U1"),
			reso("Y1"), reso("Y2"), reso("Y4"),
			comp("Y3", "crystal", "clock"),
			comp("C1"), comp("C2"), comp("C3"), comp("C4"), comp("C5"),
		},
		Nets: []*ir.Net{
			// Y1: XIN has a redundant external load cap C1 to ground; XOUT has none.
			tnet("XIN", "Y1.1", "U1.1", "C1.1"),
			tnet("XOUT", "Y1.2", "U1.2"),
			// Y2: wired straight to the driver, no external caps.
			tnet("XIN2", "Y2.1", "U1.3"),
			tnet("XOUT2", "Y2.2", "U1.4"),
			// Y3: a crystal WITH its (correct) load caps -> the resonator rule ignores it.
			tnet("XIN3", "Y3.1", "C3.1"),
			// Y4: terminal is an unresolved external net -> skipped even though C4 reaches ground.
			xext,
			// A coupling cap C5 between two signal nets (no ground leg) on a Y2 terminal is not a load cap.
			tnet("COUPLE", "Y2.1", "C5.1"),
			tnet("COUPLE2", "C5.2", "U1.5"),
			// Ground: the far leg of C1/C3/C4 plus the resonators' center pins.
			tnet("GND", "C1.2", "C3.2", "C4.2", "Y1.3", "Y2.3", "Y3.3", "Y4.3"),
		},
	}
}

func TestResonatorRedundantLoadCaps(t *testing.T) {
	fs := resonatorRedundantLoadCaps.Eval(check.NewModel(resonatorFixture()))
	if len(fs) != 1 || fs[0].Subject != "Y1" || fs[0].Kind != check.KindComponent {
		t.Fatalf("findings = %+v, want exactly one KindComponent finding on Y1", fs)
	}
	want := "ceramic resonator terminal net XIN has an external load capacitor C1 (this part integrates its load caps)"
	if fs[0].Message != want {
		t.Errorf("message = %q, want %q", fs[0].Message, want)
	}
}

// TestResonatorRedundantSilentWithoutSeededClass pins the datasheet-gate: an un-subtyped clock
// candidate (ref-des Y, no seeded ceramic_resonator class) is NOT treated as a resonator, so a
// terminal load cap on it does not fire -- it may be a crystal, which needs the cap.
func TestResonatorRedundantSilentWithoutSeededClass(t *testing.T) {
	d := &ir.Design{
		Components: []*ir.Component{
			{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "Y1", Prov: &ir.Provenance{SourceFile: "t"}}, // bare clock candidate, unseeded
			{RefDes: "C1", Prov: &ir.Provenance{SourceFile: "t"}},
		},
		Nets: []*ir.Net{
			tnet("XIN", "Y1.1", "U1.1", "C1.1"),
			tnet("GND", "C1.2", "Y1.3"),
		},
	}
	if fs := resonatorRedundantLoadCaps.Eval(check.NewModel(d)); len(fs) != 0 {
		t.Errorf("un-seeded clock candidate: want silent, got %+v", fs)
	}
}
