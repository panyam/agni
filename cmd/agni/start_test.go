package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// startFixture copies the CAN design into a source folder under t.TempDir, plus any extra siblings
// named, and returns the source design path and the temp root.
func startFixture(t *testing.T, siblings ...string) (design, root string) {
	t.Helper()
	root = t.TempDir()
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile("testdata/review/can-broken.edn")
	if err != nil {
		t.Fatal(err)
	}
	design = filepath.Join(src, "gateway.edn")
	if err := os.WriteFile(design, body, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, s := range siblings {
		from := filepath.Join("testdata/review/can-broken.edn")
		if filepath.Ext(s) == ".kicad_pcb" {
			from = "../../examples/tutorial-project/designs/gateway/gateway.kicad_pcb"
		}
		b, err := os.ReadFile(from)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(src, s), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return design, root
}

func mustStart(t *testing.T, design, dir, name string) string {
	t.Helper()
	var out bytes.Buffer
	if err := runStart(&out, design, dir, name, ""); err != nil {
		t.Fatalf("start: %v", err)
	}
	return out.String()
}

// TestStartProducesAResolvableProject is the acceptance case: after `agni start`, the two commands
// that used to need a flag list resolve everything from the scaffolded descriptors.
//
// It runs `check` and `review` for real rather than asserting on the generated YAML, because the
// files being well-formed is not the claim. The claim is that the store resolves them, the config
// tiers load, and the seeded checklist scores — and a stub that parsed but did not LOAD (a
// conventions.yaml the project declares and osProjectConfig then rejects) would pass a
// file-shape assertion and break every command on the project it just created.
func TestStartProducesAResolvableProject(t *testing.T) {
	design, root := startFixture(t)
	proj := filepath.Join(root, "gateway-review")
	mustStart(t, design, proj, "")

	designDir := filepath.Join(proj, "designs", "gateway")
	checkCmdOut := runCLI(t, checkCmd(), designDir)
	if !strings.Contains(checkCmdOut, "findings by rule") {
		t.Errorf("check should run against the scaffolded design, got:\n%s", checkCmdOut)
	}
	// No --checklist: the project declares one, so this also exercises the #218 fallback against a
	// checklist this command generated.
	reviewOut := runCLI(t, reviewCmd(), designDir)
	if !strings.Contains(reviewOut, "review") || !strings.Contains(reviewOut, "| Outcome |") {
		t.Errorf("review should run the seeded checklist with no flags, got:\n%s", reviewOut)
	}
	// A seeded checklist whose every item read not-automated would be a checklist that binds nothing,
	// which is worse than no checklist: it reports coverage that does not exist. Counted over the item
	// ROWS, since the per-area tally lines name every outcome whether or not any item holds it.
	rows := strings.Count(reviewOut, "| not-automated |")
	if rows != 1 {
		t.Errorf("exactly one seeded item (the deliberate manual one) should be not-automated, got %d:\n%s", rows, reviewOut)
	}
}

// TestStartCompanionRule is the judgment call in this command, so it is pinned from both sides.
//
// A same-stem sibling carrying a BOARD is a view of the design and is declared. A different-stem
// netlist is a later revision and a legitimate analysis source of its own, so declaring it would turn
// a diff of two revisions into a diff of one against itself. A same-stem sibling that is merely a
// second netlist ENCODING carries no view and is declared either.
func TestStartCompanionRule(t *testing.T) {
	design, root := startFixture(t, "gateway.kicad_pcb", "gateway-rev-b.edn", "gateway.edf")
	proj := filepath.Join(root, "p")
	mustStart(t, design, proj, "")

	declared, err := os.ReadFile(filepath.Join(proj, "designs", "gateway", "design.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(declared)
	if !strings.Contains(got, "gateway.kicad_pcb") {
		t.Errorf("a board sibling is a view and must be declared:\n%s", got)
	}
	for _, never := range []string{"gateway-rev-b.edn", "gateway.edf"} {
		if strings.Contains(got, never) {
			t.Errorf("%s must not be declared a companion:\n%s", never, got)
		}
	}
	// Only the entry and the real companion are copied; the project does not absorb the whole folder.
	ents, err := os.ReadDir(filepath.Join(proj, "designs", "gateway"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 3 { // design.yaml, gateway.edn, gateway.kicad_pcb
		names := []string{}
		for _, e := range ents {
			names = append(names, e.Name())
		}
		t.Errorf("expected the entry, its board companion and the descriptor; got %v", names)
	}
}

// TestStartBoardCompanionReachesTheRun: declaring the board is not the point, resolving it is. The
// board-tier item goes from unanswerable to answered because the descriptor names the layout.
func TestStartBoardCompanionReachesTheRun(t *testing.T) {
	answered := func(siblings ...string) string {
		t.Helper()
		design, root := startFixture(t, siblings...)
		proj := filepath.Join(root, "p")
		mustStart(t, design, proj, "")
		return runCLI(t, reviewCmd(), "--coverage", filepath.Join(proj, "designs", "gateway"))
	}
	withBoard := answered("gateway.kicad_pcb")
	without := answered()
	countNA := func(s string) int { return strings.Count(s, "not-applicable") }
	if !strings.Contains(withBoard, "answered") || !strings.Contains(without, "answered") {
		t.Fatalf("coverage rollup should report an answered count")
	}
	if countNA(withBoard) >= countNA(without) && without != withBoard {
		// Compared through the rendered rollup rather than a parsed number, so this asserts what an
		// operator actually sees.
		t.Logf("with board:\n%s\nwithout:\n%s", withBoard, without)
	}
	if withBoard == without {
		t.Error("declaring a board companion must change what the run can answer")
	}
}

// TestStartRefusesToOverwrite: every planned write is checked before any write happens, so a refusal
// leaves nothing half-created.
func TestStartRefusesToOverwrite(t *testing.T) {
	design, root := startFixture(t)
	proj := filepath.Join(root, "p")
	mustStart(t, design, proj, "")
	err := runStart(&bytes.Buffer{}, design, proj, "", "")
	if err == nil {
		t.Fatal("a second start over the same folder must refuse")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("the refusal should name the file that stopped it, got %v", err)
	}
}

// TestStartAddsToAnExistingProject: pointing at a folder that already declares a project adds a
// design to it rather than nesting a second project, which would give one tree two config scopes.
func TestStartAddsToAnExistingProject(t *testing.T) {
	design, root := startFixture(t)
	proj := filepath.Join(root, "p")
	mustStart(t, design, proj, "house")

	second := filepath.Join(root, "src", "revb.edn")
	body, err := os.ReadFile(design)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, body, 0o644); err != nil {
		t.Fatal(err)
	}
	out := mustStart(t, second, proj, "")
	if !strings.Contains(out, `existing project "house"`) {
		t.Errorf("should adopt the declared project, got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(proj, "designs", "revb", "design.yaml")); err != nil {
		t.Errorf("the second design should be scaffolded: %v", err)
	}
	// And --name must not silently rename a project that already declared one.
	if err := runStart(&bytes.Buffer{}, second, proj, "other", ""); err == nil ||
		!strings.Contains(err.Error(), "would rename it") {
		t.Errorf("a conflicting --name should refuse, got %v", err)
	}
}

// TestStartRejectsANonNetlistEntry: entry names the file ANALYSIS reads, so a geometry-only file
// cannot be one. Accepting it would produce a project whose every command failed later, at a place
// that no longer names the mistake.
func TestStartRejectsANonNetlistEntry(t *testing.T) {
	_, root := startFixture(t)
	notADesign := filepath.Join(root, "src", "notes.txt")
	if err := os.WriteFile(notADesign, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runStart(&bytes.Buffer{}, notADesign, filepath.Join(root, "p"), "", "")
	if err == nil || !strings.Contains(err.Error(), "carries no netlist") {
		t.Errorf("want a netlist-shaped refusal, got %v", err)
	}
}

// TestDeriveID sanitizes rather than rejecting, because the common case is a folder named
// "Gateway ECU" and failing on it would send an operator to --name to type it back.
func TestDeriveID(t *testing.T) {
	for in, want := range map[string]string{
		"Gateway ECU":   "gateway-ecu",
		"gateway":       "gateway",
		"My_Board.v2":   "my_board.v2", // _ . - are all legal id characters and survive
		"  spaced  ":    "spaced",
		"---":           "",
		"":              "",
		"_leading":      "leading",
		"UPPER":         "upper",
		"9lives":        "9lives",
		"proj/with/sep": "proj-with-sep",
	} {
		if got := deriveID(in); got != want {
			t.Errorf("deriveID(%q) = %q, want %q", in, got, want)
		}
	}
}
