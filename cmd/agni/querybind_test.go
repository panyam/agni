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

// TestQueryBindRepeatedIsAList: one name bound twice answers for both values, and the markdown view
// states the list (agni issue 1121).
func TestQueryBindRepeatedIsAList(t *testing.T) {
	const q = `component.net(?r, ?n) => ?r, ?n`
	u1 := runCLI(t, queryCmd(), bindDesign, q, "--bind", "r=U1", "--format", "csv")
	u2 := runCLI(t, queryCmd(), bindDesign, q, "--bind", "r=U2", "--format", "csv")
	both := runCLI(t, queryCmd(), bindDesign, q, "--bind", "r=U1", "--bind", "r=U2", "--format", "csv")
	rows := func(csv string) []string { return strings.Split(strings.TrimSpace(csv), "\n")[1:] }
	if len(rows(u1)) == 0 || len(rows(u2)) == 0 {
		t.Fatalf("U1 or U2 answers nothing, so the union proves nothing:\n%s\n%s", u1, u2)
	}
	if got, want := len(rows(both)), len(rows(u1))+len(rows(u2)); got != want {
		t.Errorf("--bind r=U1 --bind r=U2 answered %d rows, want %d:\n%s", got, want, both)
	}
	for _, r := range append(rows(u1), rows(u2)...) {
		if !strings.Contains(both, r) {
			t.Errorf("the list's answer lacks %q", r)
		}
	}
	md := runCLI(t, queryCmd(), bindDesign, q, "--bind", "r=U1", "--bind", "r=U2", "--format", "markdown")
	if !strings.Contains(md, "Bound: `?r in [\"U1\", \"U2\"]`") {
		t.Errorf("the markdown view does not state the list:\n%s", md)
	}
}
