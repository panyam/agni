package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/project"
)

// write drops a file with throwaway content; these tests only exercise path resolution, never a
// reader, so the bytes never matter.
func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// designFolder builds the layout the feature is about: a netlist entry, a board companion, a
// schematic companion, an undeclared later revision, and the descriptor that says which is which.
func designFolder(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "gateway.edn"), "x")
	write(t, filepath.Join(dir, "gateway-rev-b.edn"), "x")
	write(t, filepath.Join(dir, "gateway.kicad_pcb"), "x")
	write(t, filepath.Join(dir, "gateway.kicad_sch"), "x")
	write(t, filepath.Join(dir, project.DesignDescriptor), `
name: gateway
entry: gateway.edn
companions:
  - gateway.kicad_sch
  - gateway.kicad_pcb
`)
	return dir
}

// TestResolveSourceDirectoryReadsTheDeclaredEntry: naming a design used to be an error, so there was
// no way to say "this design" rather than "this file". The board tier comes from the declared board
// companion, so a netlist entry's design still runs the board-tier rules.
func TestResolveSourceDirectoryReadsTheDeclaredEntry(t *testing.T) {
	dir := designFolder(t)
	src, err := resolveSource(dir)
	if err != nil {
		t.Fatal(err)
	}
	if src.Netlist != filepath.Join(dir, "gateway.edn") {
		t.Errorf("netlist = %q, want the declared entry", src.Netlist)
	}
	if src.Board != filepath.Join(dir, "gateway.kicad_pcb") {
		t.Errorf("board = %q, want the declared board companion", src.Board)
	}
	if !strings.Contains(src.Note, "gateway.edn") || !strings.Contains(src.Note, "gateway.kicad_pcb") {
		t.Errorf("note = %q, want it to name both files that were read", src.Note)
	}
}

// TestResolveSourceCompanionRedirects: a declared companion is a VIEW of the design, so analysis
// reads the entry, while the companion still supplies the copper it alone carries (C21).
func TestResolveSourceCompanionRedirects(t *testing.T) {
	dir := designFolder(t)
	pcb := filepath.Join(dir, "gateway.kicad_pcb")
	src, err := resolveSource(pcb)
	if err != nil {
		t.Fatal(err)
	}
	if src.Netlist != filepath.Join(dir, "gateway.edn") {
		t.Errorf("netlist = %q, want the declared entry", src.Netlist)
	}
	if src.Board != pcb {
		t.Errorf("board = %q, want the named companion (it carries the copper)", src.Board)
	}
	if !strings.Contains(src.Note, "--as-named") {
		t.Errorf("note = %q, want it to say how to opt out", src.Note)
	}

	// A companion with no board reader leaves the board tier on the entry rather than pointing it at
	// a file that has no copper to give.
	src, err = resolveSource(filepath.Join(dir, "gateway.kicad_sch"))
	if err != nil {
		t.Fatal(err)
	}
	if src.Board != filepath.Join(dir, "gateway.edn") {
		t.Errorf("board = %q, want the entry for a companion carrying no board", src.Board)
	}
}

// TestResolveSourceLeavesUndeclaredSiblingsAlone is the reason companions are declared per file
// rather than inferred from "everything beside the entry": a later revision lives in the same folder
// and IS a legitimate analysis source, so redirecting it would turn a diff of two revisions into a
// diff of one against itself.
func TestResolveSourceLeavesUndeclaredSiblingsAlone(t *testing.T) {
	dir := designFolder(t)
	revB := filepath.Join(dir, "gateway-rev-b.edn")
	src, err := resolveSource(revB)
	if err != nil {
		t.Fatal(err)
	}
	if src.Netlist != revB || src.Board != revB {
		t.Fatalf("src = %+v, want the named file untouched", src)
	}
	if src.Note != "" {
		t.Errorf("note = %q, want none: nothing but the named file was read", src.Note)
	}
}

// TestResolveSourceAsNamedOptsOut: reading a companion as a netlist is a legitimate diagnostic, so
// the redirect has an escape (the tutorial project's check-views target uses it).
func TestResolveSourceAsNamedOptsOut(t *testing.T) {
	dir := designFolder(t)
	sch := filepath.Join(dir, "gateway.kicad_sch")

	readAsNamed = true
	t.Cleanup(func() { readAsNamed = false })
	src, err := resolveSource(sch)
	if err != nil {
		t.Fatal(err)
	}
	if src.Netlist != sch {
		t.Errorf("netlist = %q, want the named companion under --as-named", src.Netlist)
	}
}

// TestResolveSourceNoDescriptorIsUnchanged: every invocation that works today keeps working, and the
// .eds sibling advice survives where there is no declaration to act on.
func TestResolveSourceNoDescriptorIsUnchanged(t *testing.T) {
	dir := t.TempDir()
	eds := filepath.Join(dir, "board.eds")
	write(t, eds, "x")

	src, err := resolveSource(eds)
	if err != nil {
		t.Fatal(err)
	}
	if src.Netlist != eds || src.Note != "" {
		t.Fatalf("lone .eds: src = %+v, want the named file and no note", src)
	}

	write(t, filepath.Join(dir, "board.edn"), "x")
	src, err = resolveSource(eds)
	if err != nil {
		t.Fatal(err)
	}
	if src.Netlist != eds {
		t.Errorf("netlist = %q: with no descriptor there is nothing to redirect to", src.Netlist)
	}
	if !strings.Contains(src.Note, "board.edn") || !strings.Contains(src.Note, "authoritative") {
		t.Errorf("note = %q, want the sibling advice naming board.edn", src.Note)
	}

	// Reading the .edn itself, and a lone .eds elsewhere, stay silent; the match is case-insensitive.
	src, _ = resolveSource(filepath.Join(dir, "board.edn"))
	if src.Note != "" {
		t.Errorf(".edn input: want no note, got %q", src.Note)
	}
	up := t.TempDir()
	write(t, filepath.Join(up, "B.EDS"), "x")
	write(t, filepath.Join(up, "B.EDN"), "x")
	src, _ = resolveSource(filepath.Join(up, "B.EDS"))
	if src.Note == "" {
		t.Error(".EDS with a .EDN sibling: want the advice, got none")
	}
}

// TestResolveSourceRejectsADirectoryWithNoDescriptor: the error names the descriptor it wanted, since
// handing a directory to a reader can only produce an unsupported-extension message for something
// that is not a file at all.
func TestResolveSourceRejectsADirectoryWithNoDescriptor(t *testing.T) {
	_, err := resolveSource(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), project.DesignDescriptor) {
		t.Fatalf("error = %v, want one naming %s", err, project.DesignDescriptor)
	}
}

// TestResolveSourceMalformedDescriptorIsAnError: an operator who wrote a design.yaml and got silently
// ignored would read the resulting default behaviour as the engine agreeing with them.
func TestResolveSourceMalformedDescriptorIsAnError(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.edn"), "x")
	write(t, filepath.Join(dir, project.DesignDescriptor), "name: gateway\n") // no entry
	if _, err := resolveSource(filepath.Join(dir, "a.edn")); err == nil {
		t.Fatal("a descriptor missing its entry should fail the read, not be skipped")
	}
}

// TestResolveSourceMissingPathDefersToTheReader keeps the not-found message the one a user already
// knows rather than a second one invented by the resolver.
func TestResolveSourceMissingPathDefersToTheReader(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.edn")
	src, err := resolveSource(missing)
	if err != nil {
		t.Fatalf("err = %v, want the resolver to pass through", err)
	}
	if src.Netlist != missing {
		t.Errorf("netlist = %q, want the named path", src.Netlist)
	}
}
