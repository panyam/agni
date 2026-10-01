package main

import (
	"strings"
	"testing"
)

const bindDesign = "../../examples/tutorial-project/designs/gateway/gateway.edn"

// TestQueryBindAnswersAsTheWrittenConstant: `--bind r=U1` answers what `"U1"` written into the query
// does, and the markdown view states the binding under the query (agni issue 793).
func TestQueryBindAnswersAsTheWrittenConstant(t *testing.T) {
	bound := runCLI(t, queryCmd(), bindDesign, `component.net(?r, ?n) => ?n`, "--bind", "r=U1")
	written := runCLI(t, queryCmd(), bindDesign, `component.net("U1", ?n) => ?n`)
	if !strings.Contains(written, "PMIC_MAIN_12V0") {
		t.Fatalf("the written query does not answer U1's nets, so the comparison proves nothing:\n%s", written)
	}
	if bound != written {
		t.Errorf("--bind answered\n%s\nthe written constant answered\n%s", bound, written)
	}
	md := runCLI(t, queryCmd(), bindDesign, `component.net(?r, ?n) => ?n`, "--bind", "r=U1", "--format", "markdown")
	if !strings.Contains(md, "Bound: `?r = \"U1\"`") {
		t.Errorf("the markdown view does not state the binding:\n%s", md)
	}
}

func TestQueryBindIsRefusedWithASet(t *testing.T) {
	cmd := queryCmd()
	cmd.SetArgs([]string{bindDesign, "--set", "-", "--bind", "r=U1"})
	cmd.SetIn(strings.NewReader("queries:\n  - name: a\n    query: component.net(?r, ?n) => ?n\n"))
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "bind:") {
		t.Errorf("err = %v, want --bind refused with a pointer to the set's own bind:", err)
	}
}
