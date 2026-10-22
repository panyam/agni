package main

import (
	"bytes"
	"strings"
	"testing"
)

// runDiffBoth runs `agni diff` and returns its stdout and stderr, where a link is printed.
func runDiffBoth(t *testing.T, args ...string) (string, string) {
	t.Helper()
	cmd := diffCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff %v: %v\n%s", args, err, errOut.String())
	}
	return out.String(), errOut.String()
}

// A diff opens in the viewer as a comparison (agni issue 1126): the link names the earlier
// revision's view and the later one as what to compare against, on stderr whatever the format, so
// the json on stdout stays the DiffDesignsResponse (C31).
func TestADiffLinksToTheViewersComparison(t *testing.T) {
	for _, format := range []string{"text", "json", "csv"} {
		t.Run(format, func(t *testing.T) {
			withMount(t)
			stdout, stderr := runDiffBoth(t, "mount://demo/crystal.fires.edn", "mount://demo/crystal.passes.edn", "--format", format, "--server", urlBaseFlag)
			u := linkIn(t, stderr, "look at it: ")
			if u.Path != "/designs/demo/crystal.fires.edn/view" || u.Query().Get("against") != "demo/crystal.passes.edn" {
				t.Errorf("link %s, want the earlier revision's view against demo/crystal.passes.edn", u)
			}
			if strings.Contains(stdout, "look at it") {
				t.Errorf("the link went to stdout:\n%s", stdout)
			}
		})
	}
}

// A revision reached through a mount this run minted gets no link, and the run says why, as check's
// and trace's do (issue 626).
func TestADiffWithholdsALinkForAnUndeclaredMount(t *testing.T) {
	freshWorkspace(t)
	_, stderr := runDiffBoth(t, "testdata/conformance/crystal.fires.edn", "testdata/conformance/crystal.passes.edn", "--server", "http://127.0.0.1:1")
	if strings.Contains(stderr, "look at it:") || !strings.Contains(stderr, "no links were emitted") {
		t.Errorf("want no link and a reason:\n%s", stderr)
	}
}

// Without --server a diff prints no link and asks no server anything.
func TestADiffWithoutAServerPrintsNoLink(t *testing.T) {
	withMount(t)
	_, stderr := runDiffBoth(t, "mount://demo/crystal.fires.edn", "mount://demo/crystal.passes.edn")
	if strings.Contains(stderr, "look at") || strings.Contains(stderr, "no links") {
		t.Errorf("link talk with no --server:\n%s", stderr)
	}
}
