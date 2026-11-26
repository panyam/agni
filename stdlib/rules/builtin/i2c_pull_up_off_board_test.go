package builtin

import (
	"context"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// offBoardBus has I2C lines with no pull-up anywhere on the board. J1 is a module socket and J2 a
// cable entry. U2 is a multiplexer with a spare channel brought out to a test point.
func offBoardBus() *ir.Design {
	comp := func(ref string, attrs map[string]string) *ir.Component {
		return &ir.Component{RefDes: ref, Attributes: attrs, Prov: &ir.Provenance{SourceFile: "t"}}
	}
	d := protDesign([]*ir.Component{
		comp("U1", nil),
		comp("U2", nil),
		comp("J1", map[string]string{"Value": "Module socket 699-pin"}),
		comp("J2", map[string]string{"Value": "JST-SH 4-pin"}),
		comp("R1", map[string]string{"Value": "0R"}),
		comp("TP1", nil),
		comp("TP2", nil),
	}, nil, []*ir.Net{
		tnet("MOD_SDA", "U1.1", "J1.1"),
		tnet("HOST_SCL", "U1.2", "R1.1"),
		tnet("MOD_SCL", "R1.2", "J1.2"),
		tnet("CABLE_SDA", "U1.3", "J2.1"),
		tnet("SPARE_SCL", "U2.1", "TP1.1"),
		tnet("BUS_SDA", "U1.4", "U2.2", "TP2.1"),
	})
	classify.Stamp(d)
	classify.StampValues(d)
	return d
}

func i2cOutcomes(t *testing.T, m check.Model) map[string]check.Verdict {
	t.Helper()
	out := map[string]check.Verdict{}
	for _, v := range i2cPullUp.Eval(context.Background(), m) {
		out[v.Subjects[0].Ref] = v
	}
	if len(out) != 6 {
		t.Fatalf("i2c-pull-up judged %d nets, want all 6: %v", len(out), out)
	}
	return out
}

// A line with no pull-up on the board is not judged when the board cannot see the pull-up's absence
// (agni issue 1911). A module across an internal connector may carry it, directly or across a 0 Ohm
// link, and a line with one part and a test point has no second device to talk to. A cable entry and
// a bus with two parts on it are still judged, and they fail.
func TestI2CPullUpLeavesOffBoardLinesUnjudged(t *testing.T) {
	m := check.NewModel(offBoardBus(), check.WithIntent(internalIntent("J1")))
	got := i2cOutcomes(t, m)
	for net, says := range map[string]string{
		"MOD_SDA":   "leaves the board through J1",
		"HOST_SCL":  "leaves the board through J1",
		"MOD_SCL":   "leaves the board through J1",
		"SPARE_SCL": "only U2 and test points",
	} {
		if v := got[net]; v.Outcome != check.NotConsidered || !strings.Contains(v.Reason, says) {
			t.Errorf("%s = %s, %q, want not-considered saying %q", net, v.Outcome, v.Reason, says)
		}
	}
	for _, net := range []string{"CABLE_SDA", "BUS_SDA"} {
		if v := got[net]; v.Outcome != check.Fail {
			t.Errorf("%s = %s, %q, want fail", net, v.Outcome, v.Reason)
		}
	}
	fired := map[string]bool{}
	for _, f := range i2cPullUpSpec.Eval(context.Background(), m) {
		fired[check.EntityRef(f.Subject)] = true
	}
	if len(fired) != 2 || !fired["CABLE_SDA"] || !fired["BUS_SDA"] {
		t.Errorf("the twin fires on %v, want CABLE_SDA and BUS_SDA", fired)
	}
}

// The internal declaration is what excuses the module lines. Without it J1 is a cable entry, and the
// line fails.
func TestI2CPullUpJudgesAModuleLineWithoutTheDeclaration(t *testing.T) {
	got := i2cOutcomes(t, check.NewModel(offBoardBus()))
	for _, net := range []string{"MOD_SDA", "HOST_SCL", "MOD_SCL"} {
		if v := got[net]; v.Outcome != check.Fail {
			t.Errorf("%s = %s, %q, want fail with J1 undeclared", net, v.Outcome, v.Reason)
		}
	}
}
