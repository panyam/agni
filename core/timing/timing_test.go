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
