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
			if u.Query().Has("rename_approx") {
				t.Errorf("link %s asks for near-renames the diff did not pair", u)
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

// A diff that paired near-renames links a comparison that pairs them too, so the reader does not
// have to tick the option the command was already given.
func TestADiffLinkCarriesNearRenames(t *testing.T) {
	withMount(t)
	_, stderr := runDiffBoth(t, "mount://demo/crystal.fires.edn", "mount://demo/crystal.passes.edn", "--rename-approx", "--server", urlBaseFlag)
	if u := linkIn(t, stderr, "look at it: "); u.Query().Get("rename_approx") != "1" {
		t.Errorf("link %s, want rename_approx=1", u)
	}
}

// Both revisions ask one server, so a note about that server (an unreachable one leaves the links
// unverified) is said once, not once per revision.
func TestADiffSaysEachLinkNoteOnce(t *testing.T) {
	withMount(t)
	_, stderr := runDiffBoth(t, "mount://demo/crystal.fires.edn", "mount://demo/crystal.passes.edn", "--server", "http://127.0.0.1:1")
	seen := map[string]bool{}
	notes := 0
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "note:") {
			continue
		}
		notes++
		if seen[line] {
			t.Errorf("said twice: %s\nstderr:\n%s", line, stderr)
		}
		seen[line] = true
	}
	if notes == 0 {
		t.Fatalf("an unreachable server left no note to check:\n%s", stderr)
	}
}
