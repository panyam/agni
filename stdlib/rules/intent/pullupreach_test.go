package intent

import (
	"strings"
	"testing"
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
	// It compiles to no rule, since the model reads it rather than a rule checking it.
	if rules := Compile(d); len(rules) != 0 {
		t.Errorf("a pull-up reach compiles no rule, got %d", len(rules))
	}
}
