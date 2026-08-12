package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// checklistProject writes a project around a copy of the CAN fixture and returns the design folder.
// checklist names the file the project declares, "" to declare none (and write none, so the
// conventional review.yaml default does not resolve either).
func checklistProject(t *testing.T, root, id, checklist string) string {
	t.Helper()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	design, err := os.ReadFile("testdata/review/can-broken.edn")
	if err != nil {
		t.Fatal(err)
	}
	write("designs/d/board.edn", string(design))
	write("designs/d/design.yaml", "name: "+id+"-d\ntitle: D\nentry: board.edn\n")
	if checklist == "" {
		write("project.yaml", "name: "+id+"\ntitle: "+id+"\n")
		return filepath.Join(root, "designs", "d")
	}
	man, err := os.ReadFile("testdata/review/mini.yaml")
	if err != nil {
		t.Fatal(err)
	}
	write(checklist, string(man))
	// Declared explicitly rather than relying on the review.yaml default, so a test naming a
	// non-default file proves the DECLARATION is read and not just the convention.
	write("project.yaml", "name: "+id+"\ntitle: "+id+"\nchecklist: "+checklist+"\n")
	return filepath.Join(root, "designs", "d")
}

// runReviewCapturing runs review and returns stdout, stderr, and the error.
func runReviewCapturing(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := reviewCmd()
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), errb.String(), err
}

// TestReviewUsesProjectChecklist is the point of the change: a design whose project declares a
// checklist needs no --checklist. Before this, `agni review designs/gateway` errored out before
// reading anything, even with the project's review.yaml sitting two directories up.
func TestReviewUsesProjectChecklist(t *testing.T) {
	design := checklistProject(t, t.TempDir(), "proj", "review.yaml")
	out, errOut, err := runReviewCapturing(t, design)
	if err != nil {
		t.Fatalf("review: %v\n%s", err, errOut)
	}
	if !strings.Contains(out, "Mini ECU review") {
		t.Errorf("the project's manifest should have run, got:\n%s", out)
	}
	// Which checklist scored a run is not recoverable from the outcomes, so a checklist nobody typed
	// has to announce itself.
	if !strings.Contains(errOut, "projects/proj") || !strings.Contains(errOut, "review.yaml") {
		t.Errorf("stderr should name the project and the checklist it declared, got %q", errOut)
	}
}

// TestReviewReadsTheDeclaredChecklistNotTheDefault: the project names a file that is NOT review.yaml,
// so a run that worked by finding the conventional name rather than by reading the declaration would
// fail here.
func TestReviewReadsTheDeclaredChecklistNotTheDefault(t *testing.T) {
	design := checklistProject(t, t.TempDir(), "proj", "checklists/house.yaml")
	out, _, err := runReviewCapturing(t, design)
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if !strings.Contains(out, "Mini ECU review") {
		t.Errorf("the declared checklist should have run, got:\n%s", out)
	}
}

// TestReviewChecklistFlagWins keeps the flag meaningful on a design that DOES belong to a project.
// The flag also suppresses the resolution note, because nothing was resolved.
func TestReviewChecklistFlagWins(t *testing.T) {
	design := checklistProject(t, t.TempDir(), "proj", "review.yaml")
	_, errOut, err := runReviewCapturing(t, "--checklist", "testdata/intent/rails-checklist.yaml", design)
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if strings.Contains(errOut, "declares") {
		t.Errorf("an explicit --checklist resolves nothing, so it must emit no resolution note, got %q", errOut)
	}
}

// TestReviewChecklistErrorsAreActionable: the three no-checklist states produce three different
// messages. Collapsing them would tell an operator with a real project to pass a flag when the
// actionable fix is a line in the project.yaml they already have.
func TestReviewChecklistErrorsAreActionable(t *testing.T) {
	t.Run("no project", func(t *testing.T) {
		_, _, err := runReviewCapturing(t, "testdata/review/can-broken.edn")
		if err == nil {
			t.Fatal("a loose design with no --checklist must still error")
		}
		if !strings.Contains(err.Error(), "belongs to no project") {
			t.Errorf("message should say the design has no project, got %v", err)
		}
	})
	t.Run("project declares none", func(t *testing.T) {
		design := checklistProject(t, t.TempDir(), "bare", "")
		_, _, err := runReviewCapturing(t, design)
		if err == nil {
			t.Fatal("a project with no checklist must error")
		}
		for _, want := range []string{"bare", "checklist:", "project.yaml"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("message should name the project and the fix, missing %q: %v", want, err)
			}
		}
	})
}

// TestReviewRollupRefusesMixedChecklists guards the rollup renderer's documented assumption that
// every report shares the manifest structure. RenderAggregateMarkdown builds its traceability matrix
// from Reports[0].Areas, so two manifests would label the rows from the first design's checklist and
// fill the cells from the second's — every row looking answered, with the wrong labels.
func TestReviewRollupRefusesMixedChecklists(t *testing.T) {
	a := checklistProject(t, t.TempDir(), "proj-a", "review.yaml")
	b := checklistProject(t, t.TempDir(), "proj-b", "review.yaml")
	_, _, err := runReviewCapturing(t, a, b)
	if err == nil {
		t.Fatal("two designs resolving to different checklists must refuse rather than score one against the other's manifest")
	}
	if !strings.Contains(err.Error(), "different checklists") || !strings.Contains(err.Error(), "--checklist") {
		t.Errorf("message should name the disagreement and the way out, got %v", err)
	}
	// And --checklist is genuinely the way out, so the message is not advice that fails when followed.
	if _, _, err := runReviewCapturing(t, "--checklist", "testdata/review/mini.yaml", a, b); err != nil {
		t.Errorf("--checklist should score both against one manifest: %v", err)
	}
}

// TestReviewRollupAcceptsOneChecklist: two designs under the SAME project resolve to one checklist and
// roll up without a flag, which is the case a gate in CI is most likely to be.
func TestReviewRollupAcceptsOneChecklist(t *testing.T) {
	root := t.TempDir()
	first := checklistProject(t, root, "proj", "review.yaml")
	second := filepath.Join(root, "designs", "d2")
	if err := os.MkdirAll(second, 0o755); err != nil {
		t.Fatal(err)
	}
	design, err := os.ReadFile("testdata/review/can-broken.edn")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(second, "board.edn"), design, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(second, "design.yaml"), []byte("name: proj-d2\ntitle: D2\nentry: board.edn\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, err := runReviewCapturing(t, first, second)
	if err != nil {
		t.Fatalf("one project, two designs: %v", err)
	}
	if !strings.Contains(out, "Review rollup") {
		t.Errorf("expected a rollup, got:\n%s", out)
	}
}
