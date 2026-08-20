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

// THE SHAPE QUESTION THIS RULE EXISTS TO ANSWER. A multi-hop path must come out as ORDERED,
// TYPED entities, because the point of carrying a path is that a reader can be sent to each hop.
//
// It lives in Context and not Witness.Terms, and the distinction is load-bearing rather than
// bookkeeping: a Term is a Label and a bare string, so "pull-up=R1" leaves a consumer guessing
// whether R1 is a component, a net or a pin, and it cannot build a highlight from a guess. Every
// entry here carries the Kind that HighlightSpec joins on.
func TestPullUpPathIsOrderedTypedContext(t *testing.T) {
	v := verdictForNet(t, pullUpVerdicts(t, []string{"U1", "U2", "R1", "R2"},
		tnet("SCL", "U1.1", "R1.1"),
		tnet("SCL_ISO", "R1.2", "U2.1", "R2.1"),
		tnet("+3V3", "R2.2")), "SCL")

	if v.Outcome != check.Pass {
		t.Fatalf("the bus is held high one hop out, want pass, got %s", v.Outcome)
	}
	want := []check.ContextSubject{
		{Kind: check.KindComponent, Subject: "R1", Role: "pull-up"},
		{Kind: check.KindNet, Subject: "SCL_ISO", Role: "segment"},
		{Kind: check.KindComponent, Subject: "R2", Role: "pull-up"},
		{Kind: check.KindNet, Subject: "+3V3", Role: "rail"},
	}
	if len(v.Context) != len(want) {
		t.Fatalf("want %d ordered hops, got %d: %+v", len(want), len(v.Context), v.Context)
	}
	for i, w := range want {
		if v.Context[i] != w {
			t.Errorf("hop %d: want %+v, got %+v", i, w, v.Context[i])
		}
	}
	// The subject is not repeated in its own context, so a consumer can draw subject-as-figure over
	// context-as-ground without the figure appearing in both layers.
	for _, c := range v.Context {
		if c.Kind == check.KindNet && c.Subject == v.Subject {
			t.Errorf("the subject net %s must not also appear as its own context", v.Subject)
		}
	}
	// Everything this proof rests on is an entity, so there is no value left for a term to carry.
	// An entity duplicated into Terms would be the same fact with its type stripped off.
	if len(v.Witness.Terms) != 0 {
		t.Errorf("a path witness rests on entities, not values; want no terms, got %+v", v.Witness.Terms)
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
