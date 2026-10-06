package main

import (
	"bytes"
	"strings"
	"testing"
)

// runRootSplit runs the CLI and returns its stdout and stderr apart.
func runRootSplit(t *testing.T, args ...string) (string, string) {
	t.Helper()
	cmd := rootCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, errOut.String())
	}
	return out.String(), errOut.String()
}

const timedGateway = "../../examples/tutorial-project/designs/gateway/gateway.edn"

// TestTimingGoesToStderrAndLeavesTheAnswerAlone holds --timing to reporting where a run's time went
// (agni issue 914) on stderr, so a timed run pipes the same answer an untimed one does.
func TestTimingGoesToStderrAndLeavesTheAnswerAlone(t *testing.T) {
	freshWorkspace(t)
	plain, plainErr := runRootSplit(t, "check", timedGateway, "--format", "csv")
	timed, timedErr := runRootSplit(t, "check", timedGateway, "--format", "csv", "--timing")
	if plain != timed {
		t.Error("--timing changed the answer on stdout")
	}
	if strings.Contains(plainErr, "timing:") {
		t.Error("an untimed run printed a timing")
	}
	for _, want := range []string{"timing: total", "read.netlist", "rules", "slowest rules"} {
		if !strings.Contains(timedErr, want) {
			t.Errorf("--timing printed no %q:\n%s", want, timedErr)
		}
	}
}

// TestExplainPrintsEachQueryPlan holds query --explain to printing the evaluation's plan on stderr,
// beside the same answer.
func TestExplainPrintsEachQueryPlan(t *testing.T) {
	freshWorkspace(t)
	q := `component.class(?r, "resistor"), component.net(?r, ?n) => ?r, ?n`
	plain, _ := runRootSplit(t, "query", timedGateway, q, "--format", "csv")
	explained, errOut := runRootSplit(t, "query", timedGateway, q, "--format", "csv", "--explain")
	if plain != explained {
		t.Error("--explain changed the answer on stdout")
	}
	for _, want := range []string{"plan for " + q, "goal", "component.net"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("--explain printed no %q:\n%s", want, errOut)
		}
	}
	if strings.Contains(errOut, "timing:") {
		t.Errorf("--explain alone printed the timings too:\n%s", errOut)
	}
}
