package main

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/panyam/agni/examples/common"
)

func TestMain(m *testing.M) {
	// The defaults the test asserts against are the bundled ones, whatever a developer exported.
	os.Unsetenv(common.DesignPathEnv)
	os.Unsetenv(common.ReviewPathEnv)
	os.Exit(m.Run())
}

// The walkthrough's calls answer as walkthrough.md says they do, over the agni this repo built.
func TestTheWalkthroughAnswersAsItSays(t *testing.T) {
	agni := agniBinary()
	if agni == "agni" {
		t.Fatal("no agni to run: build ../../bin/agni (make agni) or set AGNI_BIN; examples-test builds it")
	}
	root, design, err := mountFor(defaultFolder, defaultDesign)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	s, err := connect(ctx, agni, root, design)
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	tools, err := s.tools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"designs", "look", "query", "trace", "diff", "check"} {
		if !slices.Contains(tools, want) {
			t.Errorf("tools %v lack %s", tools, want)
		}
	}
	want := map[string][]string{
		"designs": {"ex/designs/gateway", "revision: ex/designs/gateway/gateway-rev-b.edn"},
		"look":    {"net.has_test_point("},
		"query":   {"query: net.rail(?net), not net.has_test_point(?net) => ?net", "| provenance |", "no viewer link:"},
		"trace":   {"J1", "U1", "route"},
		"diff":    {"R4", "R5"},
	}
	if len(want) != len(questions) {
		t.Fatalf("%d questions, %d with expectations", len(questions), len(want))
	}
	for _, q := range questions {
		args := map[string]any{}
		for k, v := range q.args {
			args[k] = v
		}
		text, err := s.ask(ctx, q.tool, args)
		if err != nil {
			t.Errorf("%s: %v", q.step, err)
			continue
		}
		for _, w := range want[q.step] {
			if !strings.Contains(text, w) {
				t.Errorf("%s lacks %q:\n%s", q.step, w, text)
			}
		}
	}
}
