package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureUnlisted sends the CLI's unlisted-tier notes to a buffer for one test, with no descriptor
// noted yet.
func captureUnlisted(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := unlistedOut
	unlistedOut = &buf
	forgetUnlisted()
	t.Cleanup(func() {
		unlistedOut = prev
		forgetUnlisted()
	})
	return &buf
}

func forgetUnlisted() {
	unlistedSeen.Range(func(k, _ any) bool {
		unlistedSeen.Delete(k)
		return true
	})
}

// unlistedProject is the tutorial project with its profiles and params lines removed, as a project
// written before agni 1136 looks. Its profiles/ and params/ folders stay beside project.yaml.
func unlistedProject(t *testing.T) string {
	t.Helper()
	root := tutorialProject(t)
	descriptor := filepath.Join(root, "project.yaml")
	raw, err := os.ReadFile(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	kept := strings.ReplaceAll(strings.ReplaceAll(string(raw), "profiles: [profiles]\n", ""), "params: [params]\n", "")
	if kept == string(raw) {
		t.Fatal("the tutorial project.yaml no longer lists profiles and params as this test expects")
	}
	if err := os.WriteFile(descriptor, []byte(kept), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

// wantUnlistedNote checks that the note names profiles/ and params/ with the lines to add, and
// names no folder the descriptor lists (lib/, queries/) or that is no tier (boards/).
func wantUnlistedNote(t *testing.T, surface, got string) {
	t.Helper()
	if !strings.Contains(got, "profiles/ and params/ sit beside") || !strings.Contains(got, `"profiles: [profiles]"`) || !strings.Contains(got, `"params: [params]"`) {
		t.Errorf("%s noted %q; want profiles/ and params/ named with the lines that list them", surface, got)
	}
	for _, other := range []string{"lib/", "queries/", "boards/"} {
		if strings.Contains(got, other) {
			t.Errorf("%s noted %q; want no note about %s", surface, got, other)
		}
	}
	if n := strings.Count(got, "note: "); n != 1 {
		t.Errorf("%s noted %d times: %q; want once", surface, n, got)
	}
}

// A project whose project.yaml leaves out the profiles/ and params/ beside it says so on check and
// review, naming the lines to add, rather than running without them in silence (agni issue 1861).
func TestAnUnlistedTierFolderIsNotedOnCheckAndReview(t *testing.T) {
	root := unlistedProject(t)
	gw := filepath.Join(root, "boards", "gw")

	notes := captureUnlisted(t)
	freshWorkspace(t)
	if out, err := runAgni(t, "check", gw); err != nil {
		t.Fatalf("check: %v\n%s", err, out)
	}
	wantUnlistedNote(t, "check", notes.String())
	if !strings.Contains(notes.String(), filepath.Join(root, "project.yaml")) {
		t.Errorf("check noted %q; want the project.yaml named by its path, %s", notes.String(), filepath.Join(root, "project.yaml"))
	}

	notes = captureUnlisted(t)
	freshWorkspace(t)
	_, _ = runAgni(t, "review", gw)
	wantUnlistedNote(t, "review", notes.String())
}

// A project.yaml that lists every tier folder beside it gets no note.
func TestAListedTierFolderIsNotNoted(t *testing.T) {
	root := tutorialProject(t)
	notes := captureUnlisted(t)
	freshWorkspace(t)
	if out, err := runAgni(t, "check", filepath.Join(root, "boards", "gw")); err != nil {
		t.Fatalf("check: %v\n%s", err, out)
	}
	if notes.Len() != 0 {
		t.Errorf("check of a project listing its tiers noted %q; want nothing", notes.String())
	}
}

// packs push names the folders the pack leaves out, since a pack holds only what project.yaml lists.
func TestPacksPushNotesAnUnlistedTierFolder(t *testing.T) {
	root := unlistedProject(t)
	srv := designServer(t, t.TempDir(), t.TempDir())
	notes := captureUnlisted(t)
	if out, err := runPacks(t, srv.URL, "push", root); err != nil {
		t.Fatalf("packs push: %v\n%s", err, out)
	}
	wantUnlistedNote(t, "packs push", notes.String())
}
