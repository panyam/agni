package builtin

import (
	"context"
	"testing"

	"github.com/panyam/agni/core/check"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// fourHopBus is SDA_MAIN reaching +3V3 through four series resistors, one past the default reach.
// SCL rides along with a direct pull-up so the run has a net the declaration does not name.
func fourHopBus() ([]string, []*ir.Net) {
	return []string{"U1", "R1", "R2", "R3", "R4", "R9"}, []*ir.Net{
		tnet("SDA_MAIN", "U1.3", "R1.1"),
		tnet("HOP1", "R1.2", "R2.1"),
		tnet("HOP2", "R2.2", "R3.1"),
		tnet("HOP3", "R3.2", "R4.1"),
		tnet("+3V3", "R4.2", "R9.2"),
		tnet("SCL_MAIN", "U1.4", "R9.1"),
	}
}

func pullUpVerdictsWithIntent(t *testing.T, di *configpb.DesignIntent, refs []string, nets []*ir.Net) []check.Verdict {
	t.Helper()
	m := check.NewModel(&ir.Design{Components: comps(refs...), Nets: nets}, check.WithIntent(di))
	return i2cPullUpVerdicts(context.Background(), m)
}

// The acceptance for agni issue 820. The same board fails at the default reach and passes once the
// design declares that this bus's series network reaches four resistors, and each witness states the
// limit in effect for that net.
func TestDeclaredPullUpReachIsTheLimitTheWalkUses(t *testing.T) {
	refs, nets := fourHopBus()

	def := verdictForNet(t, pullUpVerdicts(t, refs, nets...), "SDA_MAIN")
	if def.Outcome != check.Fail {
		t.Fatalf("four crossings is past the default reach, want fail, got %s", def.Outcome)
	}
	if got := def.Witness.Statement; got != "no rail is reachable from SDA_MAIN through a resistor within 3 hops" {
		t.Errorf("default witness = %q", got)
	}

	declared := &configpb.DesignIntent{Nets: map[string]*configpb.NetIntent{"SDA_MAIN": {PullUpReachHops: 4}}}
	vs := pullUpVerdictsWithIntent(t, declared, refs, nets)
	v := verdictForNet(t, vs, "SDA_MAIN")
	if v.Outcome != check.Pass {
		t.Fatalf("declared reach 4 covers four crossings, want pass, got %s (%v)", v.Outcome, v.Witness)
	}
	if got, want := v.Witness.Statement, "SDA_MAIN reaches rail +3V3 through R1 then R2 then R3 then R4"; got != want {
		t.Errorf("witness = %q, want %q", got, want)
	}
	// SCL_MAIN declares nothing, so it keeps the default and its direct pull-up still passes.
	if scl := verdictForNet(t, vs, "SCL_MAIN"); scl.Outcome != check.Pass {
		t.Errorf("an undeclared net keeps the default reach, want pass, got %s", scl.Outcome)
	}

	// A declared reach NARROWER than the path fails, and the witness names the declared number
	// rather than the default, which is what tells a reader which limit the bus was held to.
	tight := &configpb.DesignIntent{Nets: map[string]*configpb.NetIntent{"SDA_MAIN": {PullUpReachHops: 2}}}
	refs2 := []string{"U1", "R1", "R2", "R3"}
	nets2 := []*ir.Net{
		tnet("SDA_MAIN", "U1.3", "R1.1"),
		tnet("HOP1", "R1.2", "R2.1"),
		tnet("HOP2", "R2.2", "R3.1"),
		tnet("+3V3", "R3.2"),
	}
	if d := verdictForNet(t, pullUpVerdicts(t, refs2, nets2...), "SDA_MAIN"); d.Outcome != check.Pass {
		t.Fatalf("positive control: three crossings pass at the default, got %s", d.Outcome)
	}
	f := verdictForNet(t, pullUpVerdictsWithIntent(t, tight, refs2, nets2), "SDA_MAIN")
	if f.Outcome != check.Fail {
		t.Fatalf("declared reach 2 does not cover three crossings, want fail, got %s", f.Outcome)
	}
	if got := f.Witness.Statement; got != "no rail is reachable from SDA_MAIN through a resistor within 2 hops" {
		t.Errorf("declared-limit witness = %q", got)
	}
	if len(f.Witness.Terms) != 1 || f.Witness.Terms[0] != (check.WitnessTerm{Label: "hop limit", Value: "2"}) {
		t.Errorf("hop limit term = %+v, want the declared 2", f.Witness.Terms)
	}
}

// The declaration reaches every walk, not only i2c-pull-up: the redundant-pull-up count walks with
// the same limit, so a second pull-up four resistors out is counted once the reach covers it.
func TestDeclaredPullUpReachReachesTheCountRules(t *testing.T) {
	refs := []string{"U1", "R1", "R2", "R3", "R4", "R5"}
	nets := []*ir.Net{
		tnet("SDA", "U1.1", "R1.1", "R5.1"),
		tnet("A", "R1.2", "R2.1"),
		tnet("B", "R2.2", "R3.1"),
		tnet("C", "R3.2", "R4.1"),
		tnet("+3V3", "R4.2", "R5.2"),
	}
	count := func(di *configpb.DesignIntent) check.Outcome {
		m := check.NewModel(&ir.Design{Components: comps(refs...), Nets: nets}, check.WithIntent(di))
		return verdictForNet(t, i2cRedundantPullUpVerdicts(context.Background(), m), "SDA").Outcome
	}
	if got := count(nil); got != check.Pass {
		t.Fatalf("at the default reach only R5 is a pull-up, want pass, got %s", got)
	}
	di := &configpb.DesignIntent{Nets: map[string]*configpb.NetIntent{"SDA": {PullUpReachHops: 4}}}
	if got := count(di); got != check.Fail {
		t.Errorf("at reach 4, R4 and R5 both pull SDA to +3V3, want fail, got %s", got)
	}
}

// The acceptance for agni issue 821. A direct pull-up and a second reached through a series resistor
// are both on the verdict, the shorter first, each as a path term, and the context lists each entity
// once.
func TestPullUpPassListsEveryPath(t *testing.T) {
	v := verdictForNet(t, pullUpVerdicts(t, []string{"U1", "R1", "R2", "R3"},
		tnet("SCL", "U1.1", "R2.1", "R3.1"),
		tnet("SCL_ISO", "R2.2", "R1.1"),
		tnet("+3V3", "R1.2", "R3.2")), "SCL")

	if v.Outcome != check.Pass {
		t.Fatalf("want pass, got %s", v.Outcome)
	}
	if got, want := v.Witness.Statement, "SCL reaches rail +3V3 through R3, and rail +3V3 through R2 then R1"; got != want {
		t.Errorf("statement = %q, want %q", got, want)
	}
	wantTerms := []check.WitnessTerm{
		{Label: "path", Value: "SCL -> [R3] -> +3V3"},
		{Label: "path", Value: "SCL -> [R2] -> SCL_ISO -> [R1] -> +3V3"},
	}
	if len(v.Witness.Terms) != len(wantTerms) {
		t.Fatalf("terms = %+v, want %+v", v.Witness.Terms, wantTerms)
	}
	for i, w := range wantTerms {
		if v.Witness.Terms[i] != w {
			t.Errorf("term %d = %+v, want %+v", i, v.Witness.Terms[i], w)
		}
	}
	wantCtx := []check.ContextSubject{
		{Entity: check.Entity{Kind: check.KindComponent, Ref: "R3"}, Role: "pull-up"},
		{Entity: check.Entity{Kind: check.KindNet, Ref: "+3V3"}, Role: "rail"},
		{Entity: check.Entity{Kind: check.KindComponent, Ref: "R2"}, Role: "pull-up"},
		{Entity: check.Entity{Kind: check.KindNet, Ref: "SCL_ISO"}, Role: "segment"},
		{Entity: check.Entity{Kind: check.KindComponent, Ref: "R1"}, Role: "pull-up"},
	}
	if len(v.Context) != len(wantCtx) {
		t.Fatalf("context = %+v, want %+v", v.Context, wantCtx)
	}
	for i, w := range wantCtx {
		if v.Context[i] != w {
			t.Errorf("context %d = %+v, want %+v", i, v.Context[i], w)
		}
	}
}

// Pull-ups to two rails are two paths of one hop each, ordered by resistor, and each names its rail.
// Two parallel resistors to one rail (the 1091 fixture's shape) are two paths too.
func TestPullUpPassListsPathsToEachRail(t *testing.T) {
	for _, tc := range []struct {
		name      string
		nets      []*ir.Net
		statement string
		terms     []string
	}{
		{
			name: "two rails",
			nets: []*ir.Net{
				tnet("SDA", "U1.1", "R_B.1", "R_A.1"),
				tnet("+5V", "R_B.2"),
				tnet("+3V3", "R_A.2"),
			},
			statement: "SDA reaches rail +3V3 through R_A, and rail +5V through R_B",
			terms:     []string{"SDA -> [R_A] -> +3V3", "SDA -> [R_B] -> +5V"},
		},
		{
			name: "parallel to one rail",
			nets: []*ir.Net{
				tnet("SDA", "U1.1", "R_B.1", "R_A.1"),
				tnet("+3V3", "R_A.2", "R_B.2"),
			},
			statement: "SDA reaches rail +3V3 through R_A, and rail +3V3 through R_B",
			terms:     []string{"SDA -> [R_A] -> +3V3", "SDA -> [R_B] -> +3V3"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := verdictForNet(t, pullUpVerdicts(t, []string{"U1", "R_A", "R_B"}, tc.nets...), "SDA")
			if v.Outcome != check.Pass {
				t.Fatalf("want pass, got %s", v.Outcome)
			}
			if v.Witness.Statement != tc.statement {
				t.Errorf("statement = %q, want %q", v.Witness.Statement, tc.statement)
			}
			if len(v.Witness.Terms) != len(tc.terms) {
				t.Fatalf("terms = %+v, want %d paths", v.Witness.Terms, len(tc.terms))
			}
			for i, want := range tc.terms {
				if got := v.Witness.Terms[i]; got.Label != "path" || got.Value != want {
					t.Errorf("term %d = %+v, want path %q", i, got, want)
				}
			}
		})
	}
}
