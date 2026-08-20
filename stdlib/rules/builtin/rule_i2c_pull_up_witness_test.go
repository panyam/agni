package builtin

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Proof-on-pass for a CONNECTIVITY rule. The datasheet rules prove a pass with values; this one
// proves it with a path, and the point of these tests is that one Witness carries both without
// changing shape. The failure mode is the same one build/evidence.md names: "a witness is present"
// cannot fail, so what is asserted here is that the witness TRACKS THE TOPOLOGY.

func verdictForNet(t *testing.T, vs []check.Verdict, net string) check.Verdict {
	t.Helper()
	for _, v := range vs {
		if v.Subject == net {
			return v
		}
	}
	t.Fatalf("no verdict for net %s in %d verdicts", net, len(vs))
	return check.Verdict{}
}

func pullUpVerdicts(t *testing.T, refs []string, nets ...*ir.Net) []check.Verdict {
	t.Helper()
	return i2cPullUpVerdicts(check.NewModel(&ir.Design{Components: comps(refs...), Nets: nets}))
}

// THE PROPERTY. A pass must name the resistor and the rail it rests on, and changing either must
// change what the witness says. A witness that did not read the topology would produce the same
// string for both halves and fail the comparison at the end.
func TestPullUpWitnessTracksTheTopology(t *testing.T) {
	viaR1 := verdictForNet(t, pullUpVerdicts(t, []string{"U1", "R1"},
		tnet("SCL", "U1.1", "R1.1"), tnet("+3V3", "R1.2")), "SCL")
	viaR7 := verdictForNet(t, pullUpVerdicts(t, []string{"U1", "R7"},
		tnet("SCL", "U1.1", "R7.1"), tnet("+5V", "R7.2")), "SCL")

	for _, c := range []struct {
		name string
		v    check.Verdict
		want []string
	}{
		{"R1 to +3V3", viaR1, []string{"R1", "+3V3"}},
		{"R7 to +5V", viaR7, []string{"R7", "+5V"}},
	} {
		if c.v.Outcome != check.Pass {
			t.Fatalf("%s: the bus is held high, want pass, got %s", c.name, c.v.Outcome)
		}
		if c.v.Witness == nil {
			t.Fatalf("%s: a pass must carry the path that justifies it", c.name)
		}
		for _, w := range c.want {
			if !strings.Contains(c.v.Witness.Statement, w) {
				t.Errorf("%s: witness must name %q, got %q", c.name, w, c.v.Witness.Statement)
			}
		}
	}
	if viaR1.Witness.Statement == viaR7.Witness.Statement {
		t.Errorf("witness does not track the topology: identical statement %q for two different pull-ups",
			viaR1.Witness.Statement)
	}
}

// THE SHAPE QUESTION THIS RULE EXISTS TO ANSWER. Witness.Terms was built as an ordered open list
// rather than a measured/limit pair, on the argument that the next witness family would be a path.
// This is that family: a multi-hop path must come out as the ordered hops, with the final net
// labelled as the rail, and no term type had to be added to carry it.
func TestPullUpWitnessCarriesTheOrderedPath(t *testing.T) {
	v := verdictForNet(t, pullUpVerdicts(t, []string{"U1", "U2", "R1", "R2"},
		tnet("SCL", "U1.1", "R1.1"),
		tnet("SCL_ISO", "R1.2", "U2.1", "R2.1"),
		tnet("+3V3", "R2.2")), "SCL")

	if v.Outcome != check.Pass {
		t.Fatalf("the bus is held high one hop out, want pass, got %s", v.Outcome)
	}
	want := []check.WitnessTerm{
		{Label: "net", Value: "SCL"},
		{Label: "pull-up", Value: "R1"},
		{Label: "net", Value: "SCL_ISO"},
		{Label: "pull-up", Value: "R2"},
		{Label: "rail", Value: "+3V3"},
	}
	if len(v.Witness.Terms) != len(want) {
		t.Fatalf("want %d ordered terms, got %d: %+v", len(want), len(v.Witness.Terms), v.Witness.Terms)
	}
	for i, w := range want {
		if v.Witness.Terms[i] != w {
			t.Errorf("term %d: want %+v, got %+v", i, w, v.Witness.Terms[i])
		}
	}
}

// A FAIL CARRIES ITS WITNESS TOO. "No rail within 3 hops" rests on the hop limit, and a reader has
// to see it: a pull-up sitting four hops away is a different situation from no pull-up at all, and
// the bare finding cannot tell them apart.
func TestFailingPullUpStatesTheHopLimit(t *testing.T) {
	v := verdictForNet(t, pullUpVerdicts(t, []string{"U1", "R1", "R2", "R3", "R4"},
		tnet("SCL", "U1.1", "R1.1"),
		tnet("SEG1", "R1.2", "R2.1"),
		tnet("SEG2", "R2.2", "R3.1"),
		tnet("SEG3", "R3.2", "R4.1"),
		tnet("+3V3", "R4.2")), "SCL")

	if v.Outcome != check.Fail {
		t.Fatalf("four crossings is past the bound, want fail, got %s", v.Outcome)
	}
	if v.Witness == nil {
		t.Fatal("a fail must say what it looked for; silence is what this work removes")
	}
	if !strings.Contains(v.Witness.Statement, "3") {
		t.Errorf("witness must state the hop limit it searched to, got %q", v.Witness.Statement)
	}
	if v.Finding == nil {
		t.Error("a failing verdict must carry the finding the check path reports")
	}
}

// THE CONSIDERED SET IS TOTAL. Every I2C net answers exactly once, passing or failing, so the
// verdict list is the set of nets this rule was applied to and not just the ones that failed.
func TestEveryI2CNetAnswersOnce(t *testing.T) {
	vs := pullUpVerdicts(t, []string{"U1", "R1"},
		tnet("SCL", "U1.1", "R1.1"),
		tnet("+3V3", "R1.2"),
		tnet("SDA", "U1.2"),
		tnet("PLAIN_SIGNAL", "U1.3")) // not I2C: must not appear

	seen := map[string]int{}
	for _, v := range vs {
		seen[v.Subject]++
		if v.Witness == nil {
			t.Errorf("%s: every verdict rests on something, pass or fail", v.Subject)
		}
	}
	if seen["SCL"] != 1 || seen["SDA"] != 1 {
		t.Errorf("both I2C nets must answer exactly once, got %+v", seen)
	}
	if seen["PLAIN_SIGNAL"] != 0 {
		t.Error("a non-I2C net is out of scope and must not be in the considered set")
	}
	// Positive control: the two nets really did reach opposite outcomes, so "answers once" was
	// asserted over a pass AND a fail rather than two of a kind.
	if verdictForNet(t, vs, "SCL").Outcome == verdictForNet(t, vs, "SDA").Outcome {
		t.Fatal("fixture must produce one pass and one fail or this proves less than it looks")
	}
}
