package timing

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestARequestNobodyTimesRecordsNothing(t *testing.T) {
	ctx := context.Background()
	Begin(ctx, "rules")()
	Cached(ctx, "model", "built")
	Ruled(ctx, "r", time.Second)
	Planned(ctx, "q", "plan")
	if From(ctx) != nil || Explaining(ctx) {
		t.Fatal("a context with no recorder reports one")
	}
}

func TestASnapshotOrdersWhatItRecorded(t *testing.T) {
	r := New()
	ctx := With(context.Background(), r)
	endOuter := Begin(ctx, "build.model")
	time.Sleep(2 * time.Millisecond)
	Begin(ctx, "read.board")()
	endOuter()
	Cached(ctx, "design", "memory")
	Cached(ctx, "model", "built")
	Ruled(ctx, "fast", time.Millisecond)
	Ruled(ctx, "slow", time.Second)
	s := r.Snapshot()
	if len(s.Stages) != 2 || s.Stages[0].Name != "build.model" || s.Stages[1].Name != "read.board" {
		t.Errorf("stages = %+v, want build.model then read.board, by start", s.Stages)
	}
	if st, ok := s.Slowest(); !ok || st.Name != "build.model" {
		t.Errorf("slowest = %+v, want the widest span, build.model", st)
	}
	if len(s.Rules) != 2 || s.Rules[0].Name != "slow" {
		t.Errorf("rules = %+v, want the slowest first", s.Rules)
	}
	text := s.Text(1)
	for _, want := range []string{"build.model", "cache: design=memory model=built", "slowest rules (1 of 2)", "slow"} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "fast") {
		t.Errorf("text lists a rule past its cap:\n%s", text)
	}
}

func TestAPlanIsRecordedOnlyWhenAsked(t *testing.T) {
	r := New()
	ctx := With(context.Background(), r)
	if Explaining(ctx) {
		t.Fatal("a recorder that did not ask for plans reports explaining")
	}
	r.Explain = true
	if !Explaining(ctx) {
		t.Fatal("a recorder that asked for plans does not report explaining")
	}
	Planned(ctx, "q", "goal\n  ran")
	if p := r.Snapshot().Plans; len(p) != 1 || p[0].Query != "q" {
		t.Errorf("plans = %+v", p)
	}
}

// TestARecorderReadsMemoryOnlyWhenAsked holds the memory figures to the request that asked for them
// (agni issue 994), since reading them stops the world and a timed request that did not ask must not
// pay for it.
func TestARecorderReadsMemoryOnlyWhenAsked(t *testing.T) {
	for _, asked := range []bool{true, false} {
		r := New()
		r.Memory = asked
		ctx := With(context.Background(), r)
		Begin(ctx, "read")()
		st := r.Snapshot().Stages
		if len(st) != 1 {
			t.Fatalf("recorded %d stages, want 1", len(st))
		}
		if got := st[0].Sys > 0 && st[0].HeapAlloc > 0; got != asked {
			t.Errorf("Memory %v: stage memory sys %d heap %d, want it recorded only when asked", asked, st[0].Sys, st[0].HeapAlloc)
		}
	}
}

// TestARuleIsChargedWhatItAllocatesWhenMemoryIsAsked covers BeginRule (agni issue 994).
func TestARuleIsChargedWhatItAllocatesWhenMemoryIsAsked(t *testing.T) {
	for _, asked := range []bool{true, false} {
		r := New()
		r.Memory = asked
		done := BeginRule(With(context.Background(), r))
		sink = make([]byte, 8<<20)
		done("big")
		rules := r.Snapshot().Rules
		if len(rules) != 1 || rules[0].Name != "big" {
			t.Fatalf("rules = %+v, want the one rule", rules)
		}
		if got := rules[0].Alloc >= 8<<20; got != asked {
			t.Errorf("Memory %v: charged %d bytes for an 8 MB allocation", asked, rules[0].Alloc)
		}
	}
}

var sink []byte
