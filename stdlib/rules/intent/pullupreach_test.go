package intent

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// A negative reach would let the walk cross nothing, so a bus with its pull-up wired straight to the
// rail would fail. That is wrong on every design, so it is refused at load, by Parse and by the
// ParseProto route --intent-path takes, with the field named.
func TestPullUpReachRefusesANegativeValue(t *testing.T) {
	doc := []byte("name: t\nintent:\n  nets:\n    SDA: {pull_up_reach_hops: -1}\n")
	if _, err := Parse(doc); err == nil || !strings.Contains(err.Error(), "pull_up_reach_hops") {
		t.Errorf("Parse: want an error naming pull_up_reach_hops, got %v", err)
	}
	if _, err := ParseProto(doc); err == nil || !strings.Contains(err.Error(), "pull_up_reach_hops") {
		t.Errorf("ParseProto: want an error naming pull_up_reach_hops, got %v", err)
	}
}

// A reach is a fact about a net in its own right, so a net declaring nothing else still loads, and
// the declaration carries it in net-name order.
func TestPullUpReachAloneIsADeclaration(t *testing.T) {
	d, err := Parse([]byte("name: t\nintent:\n  nets:\n" +
		"    SDA_MAIN: {pull_up_reach_hops: 4}\n" +
		"    SCL_MAIN: {pull_up_reach_hops: 5}\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []PullUpReach{{Net: "SCL_MAIN", Hops: 5}, {Net: "SDA_MAIN", Hops: 4}}
	if len(d.PullUpReach) != len(want) {
		t.Fatalf("got %+v, want %+v", d.PullUpReach, want)
	}
	for i, w := range want {
		if d.PullUpReach[i] != w {
			t.Errorf("reach %d = %+v, want %+v", i, d.PullUpReach[i], w)
		}
	}
	// The model reads it, so the one rule it compiles only confirms each named net exists.
	if rules := Compile(d); len(rules) != 1 || rules[0].Name != RulePullUpReachDeclared {
		t.Errorf("a pull-up reach compiles exactly %s, got %d rules", RulePullUpReachDeclared, len(rules))
	}
}

// A declaration naming a net the design has passes, and one naming a net it lacks fails, since the
// reach is keyed by the exact name and a misspelling would otherwise change nothing while reading as a
// declaration that took effect (agni issue 1165).
func TestPullUpReachDeclared(t *testing.T) {
	decl := declOf(t, `
name: bus
intent:
  nets:
    SDA_MAIN: {pull_up_reach_hops: 4}
    SDA-MAIN: {pull_up_reach_hops: 4}
`)
	d := &ir.Design{Nets: []*ir.Net{{Name: "SDA_MAIN", Id: "n1"}}}
	got := map[string]check.Verdict{}
	for _, v := range check.RunVerdictsBackground(check.NewModel(d), Compile(decl)) {
		if v.Rule == RulePullUpReachDeclared {
			got[check.EntityRef(v.Subjects[0])] = v
		}
	}
	if len(got) != 2 {
		t.Fatalf("want a verdict per declared net, got %v", got)
	}
	if v := got["SDA_MAIN"]; v.Outcome != check.Pass || v.Subjects[0].NetID != "n1" {
		t.Errorf("SDA_MAIN: outcome %v, net id %q, want a pass on the design's net", v.Outcome, v.Subjects[0].NetID)
	}
	miss := got["SDA-MAIN"]
	if miss.Outcome != check.Fail || miss.Finding == nil {
		t.Fatalf("SDA-MAIN: outcome %v, want a failing finding", miss.Outcome)
	}
	if want := "SDA-MAIN declares a pull-up reach of 4, and the design has no net SDA-MAIN"; miss.Finding.Message != want {
		t.Errorf("message = %q, want %q", miss.Finding.Message, want)
	}
}
