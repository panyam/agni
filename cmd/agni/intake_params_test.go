package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeProjectWithParams builds the smallest project that reproduces agni issue 474: a project.yaml
// so the design resolves to a project, a params/ directory the project therefore composes by default,
// and a design under designs/ carrying an MPN that corpus does not seed.
func writeProjectWithParams(t *testing.T, root string) {
	t.Helper()
	mk := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("project.yaml", "name: board\ntitle: Test project\n")
	mk("params/seeded.textproto", "mpn: \"ACME-SEEDED\"\n")
	mk("designs/board/design.yaml", "name: board\ntitle: Test board\nentry: board.edn\n")
	mk("designs/board/board.edn", mpnEDN)
}

func intakeOut(t *testing.T, args ...string) string {
	t.Helper()
	cmd := rootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out.String())
	}
	return out.String()
}

// TestIntakeReadsTheProjectsParams is the issue: check and review compose the params/ a project
// declares, intake took its corpus from --params alone. Inside a project the datasheet-gap section
// was absent rather than empty unless you named a directory the project already names.
func TestIntakeReadsTheProjectsParams(t *testing.T) {
	proj := t.TempDir()
	writeProjectWithParams(t, proj)
	t.Chdir(proj)

	got := intakeOut(t, "intake", "designs/board/board.edn")
	if !strings.Contains(got, "Datasheet gaps") {
		t.Fatalf("no gap section with the project's own params/ in place:\n%s", got)
	}
	if !strings.Contains(got, "ACME-UNSEEDED") {
		t.Errorf("the unseeded MPN is not in the queue:\n%s", got)
	}
}

// TestIntakeParamsFlagStillWorksOutsideAProject: the flag is the route for a loose design, and
// widening the project path must not have closed it.
func TestIntakeParamsFlagStillWorksOutsideAProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "p"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "p", "seeded.textproto"), []byte("mpn: \"ACME-SEEDED\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "board.edn"), []byte(mpnEDN), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	if got := intakeOut(t, "intake", "board.edn", "--params", "p"); !strings.Contains(got, "ACME-UNSEEDED") {
		t.Errorf("--params did not populate the queue for a design in no project:\n%s", got)
	}
	if got := intakeOut(t, "intake", "board.edn"); strings.Contains(got, "Datasheet gaps") {
		t.Errorf("a loose design with no flag has no corpus and must print no queue:\n%s", got)
	}
}

// TestIntakeAndCheckAgreeOnTheCorpus: both commands resolve the same design to the same project, so a
// corpus one of them sees and the other does not is the drift this closes. check reads it through the
// service overlay; intake now reads the same overlay rather than a second resolution of its own.
func TestIntakeAndCheckAgreeOnTheCorpus(t *testing.T) {
	proj := t.TempDir()
	writeProjectWithParams(t, proj)
	t.Chdir(proj)

	withFlag := intakeOut(t, "intake", "designs/board/board.edn", "--params", "params")
	without := intakeOut(t, "intake", "designs/board/board.edn")
	if withFlag != without {
		t.Errorf("naming the project's own params/ changed the report, so the flag and the project disagree:\n--- with flag ---\n%s\n--- without ---\n%s", withFlag, without)
	}
}

const mpnEDN = `(edif BOARD
  (edifVersion 2 0 0)
  (edifLevel 0)
  (keywordMap (keywordLevel 0))
  (library PARTS
    (edifLevel 0)
    (technology (numberDefinition))
    (cell CAP (cellType generic)
      (view V (viewType netlist)
        (interface (port A (direction INOUT)) (port B (direction INOUT))))))
  (library WORK
    (edifLevel 0)
    (technology (numberDefinition))
    (cell TOP (cellType generic)
      (view V (viewType netlist)
        (interface)
        (contents
          (instance IC1 (viewRef V (cellRef CAP (libraryRef PARTS))) (designator "C1")
            (property MPN (string "ACME-UNSEEDED")))
          (instance IC2 (viewRef V (cellRef CAP (libraryRef PARTS))) (designator "C2")
            (property MPN (string "ACME-UNSEEDED")))
          (instance IC3 (viewRef V (cellRef CAP (libraryRef PARTS))) (designator "C3")
            (property MPN (string "ACME-SEEDED")))
          (net N1 (joined
            (portRef A (instanceRef IC1))
            (portRef A (instanceRef IC2))
            (portRef A (instanceRef IC3)))))))) 
  (design BOARD (cellRef TOP (libraryRef WORK))))
`
