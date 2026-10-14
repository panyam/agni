package service

import (
	"testing"
	"time"

	"github.com/panyam/agni/core/timing"
)

// TestARequestListsTheRulesThatAllocateMost covers the timing a memory-asking request returns (agni
// issue 994): beside the slowest rules, the ones that allocated most, since the rule raising a
// worker's memory is often fast.
func TestARequestListsTheRulesThatAllocateMost(t *testing.T) {
	var rules []timing.RuleTime
	for i := 0; i < timedRules+5; i++ {
		rules = append(rules, timing.RuleTime{Name: "slow" + string(rune('a'+i)), Dur: time.Duration(100-i) * time.Millisecond})
	}
	rules = append(rules, timing.RuleTime{Name: "allocator", Dur: time.Millisecond, Alloc: 900 << 20})
	got := TimingProto("p", timing.Snapshot{Rules: rules}).GetRules()
	if len(got) != timedRules+1 {
		t.Fatalf("listed %d rules, want the %d slowest and the allocator", len(got), timedRules)
	}
	last := got[len(got)-1]
	if last.GetRule() != "allocator" || last.GetAllocBytes() != 900<<20 {
		t.Errorf("last listed rule is %s with %d bytes, want the allocator with 900 MB", last.GetRule(), last.GetAllocBytes())
	}
	if n := len(TimingProto("p", timing.Snapshot{Rules: rules[:timedRules+5]}).GetRules()); n != timedRules {
		t.Errorf("with no allocation recorded, listed %d rules, want %d", n, timedRules)
	}
}
