package server

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/panyam/agni/core/timing"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// slowCall runs one request through the timing interceptor, its handler spending a stage and a rule.
func slowCall(t *testing.T, tm Timing) {
	t.Helper()
	next := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		end := timing.Begin(ctx, "rules")
		time.Sleep(2 * time.Millisecond)
		end()
		timing.Ruled(ctx, "copper-clearance", 2*time.Millisecond)
		timing.Cached(ctx, "model", "built")
		return connect.NewResponse(&webapi.ListRulesResponse{}), nil
	}
	if _, err := tm.Interceptor()(next)(context.Background(), connect.NewRequest(&webapi.ListRulesRequest{})); err != nil {
		t.Fatal(err)
	}
}

// TestASlowRequestIsLoggedWithWhatTookTheTime holds --slow-request to one line naming the stage that
// dominated, which tier answered each read, and the slowest rules: the issue's two motivating causes
// were a stage that dominates and a cache layer missing on every request (agni issue 914).
func TestASlowRequestIsLoggedWithWhatTookTheTime(t *testing.T) {
	var lines []string
	log := func(format string, args ...any) {
		lines = append(lines, strings.TrimSpace(fmt.Sprintf(format, args...)))
	}
	slowCall(t, Timing{Slow: time.Nanosecond, Log: log})
	if len(lines) != 1 {
		t.Fatalf("logged %d lines, want one: %q", len(lines), lines)
	}
	for _, want := range []string{"slow request:", "slowest stage rules", "cache model=built", "slowest rules copper-clearance"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("the line lacks %q: %s", want, lines[0])
		}
	}
	lines = nil
	slowCall(t, Timing{Slow: time.Hour, Log: log})
	if len(lines) != 0 {
		t.Errorf("a request under the threshold was logged: %q", lines)
	}
}

// TestASlowEvaluationIsLoggedWithItsPlan holds the log line to carrying the plan when evaluating a
// query was the slowest stage, and only then.
func TestASlowEvaluationIsLoggedWithItsPlan(t *testing.T) {
	s := timing.Snapshot{
		Total:  time.Second,
		Stages: []timing.Stage{{Name: "evaluate", Dur: 900 * time.Millisecond}, {Name: "read.netlist", Dur: 50 * time.Millisecond}},
		Plans:  []timing.Plan{{Query: "net.reaches(?a, ?b) => ?a", Text: "goal\n  net.reaches(?a, ?b)"}},
	}
	line := SlowLine("/agni.v1.webapi.QueryService/RunQuery", time.Millisecond, s)
	if !strings.Contains(line, "plan for net.reaches(?a, ?b) => ?a") || !strings.Contains(line, "    goal") {
		t.Errorf("a slow evaluation's line carries no plan:\n%s", line)
	}
	s.Stages[0].Name = "read.board"
	if line := SlowLine("x", time.Millisecond, s); strings.Contains(line, "plan for") {
		t.Errorf("a line whose slowest stage was a read carries a plan:\n%s", line)
	}
}
