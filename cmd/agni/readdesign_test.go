package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/internal/projects"
)

// resolve is the test-side spelling of what every CLI read does: build a resolver from the flag and
// ask the ProjectService which artifacts to open.
func resolve(t *testing.T, path string) (designSource, error) {
	t.Helper()
	ws, err := workspace()
	if err != nil {
		t.Fatal(err)
	}
	return newDesignResolver(ws).Resolve(context.Background(), path)
}

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
	write(t, filepath.Join(dir, projects.DesignDescriptor), `
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
	src, err := resolve(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	if localOf(src.NetlistURI) != filepath.Join(dir, "gateway.edn") {
		t.Errorf("netlist = %q, want the declared entry", localOf(src.NetlistURI))
	}
	if localOf(src.BoardURI) != filepath.Join(dir, "gateway.kicad_pcb") {
		t.Errorf("board = %q, want the declared board companion", localOf(src.BoardURI))
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
	src, err := resolve(t, pcb)
	if err != nil {
		t.Fatal(err)
	}
	if localOf(src.NetlistURI) != filepath.Join(dir, "gateway.edn") {
		t.Errorf("netlist = %q, want the declared entry", localOf(src.NetlistURI))
	}
	if localOf(src.BoardURI) != pcb {
		t.Errorf("board = %q, want the named companion (it carries the copper)", localOf(src.BoardURI))
	}
	if !strings.Contains(src.Note, "--as-named") {
		t.Errorf("note = %q, want it to say how to opt out", src.Note)
	}

	// Naming a companion that carries NO copper still gets the board tier, from the design's declared
	// board. The design's board is the design's board whichever of its views you point at, and the
	// alternative is reporting every board rule not-applicable while the board file sits right there.
	// It is more than was asked for, so the note has to say so.
	src, err = resolve(t, filepath.Join(dir, "gateway.kicad_sch"))
	if err != nil {
		t.Fatal(err)
	}
	if localOf(src.BoardURI) != filepath.Join(dir, "gateway.kicad_pcb") {
		t.Errorf("board = %q, want the design's declared board companion", localOf(src.BoardURI))
	}
	if !strings.Contains(src.Note, "board geometry from gateway.kicad_pcb") {
		t.Errorf("note = %q, want it to name the board it pulled in unasked", src.Note)
	}
}

// TestResolveSourceNoteOnlyNamesUnaskedArtifacts: the note exists to report what was read but not
// asked for, so listing the very file the caller named makes it noise that trains people to skip it.
// The comparison has to be ref-against-ref; comparing a ref to the typed path never matches and
// every tier then reads as unasked.
func TestResolveSourceNoteOnlyNamesUnaskedArtifacts(t *testing.T) {
	dir := designFolder(t)
	src, err := resolve(t, filepath.Join(dir, "gateway.kicad_pcb"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(src.Note, "board geometry from") {
		t.Errorf("note = %q: the board was the file named, so it was not pulled in unasked", src.Note)
	}
	if !strings.Contains(src.Note, "sheets from gateway.kicad_sch") {
		t.Errorf("note = %q, want the schematic tier, which WAS pulled in unasked", src.Note)
	}
	// The descriptor is named by the design's own mount-relative path. For this fixture the mount is
	// minted at the design folder, so that is just the bare descriptor name — short, and the same
	// spelling whether the caller typed a path or a URI.
	if !strings.Contains(src.Note, projects.DesignDescriptor) {
		t.Errorf("note = %q, want it to name the descriptor", src.Note)
	}
}

// TestResolveSourceLeavesUndeclaredSiblingsAlone is the reason companions are declared per file
// rather than inferred from "everything beside the entry": a later revision lives in the same folder
// and IS a legitimate analysis source, so redirecting it would turn a diff of two revisions into a
// diff of one against itself.
func TestResolveSourceLeavesUndeclaredSiblingsAlone(t *testing.T) {
	dir := designFolder(t)
	revB := filepath.Join(dir, "gateway-rev-b.edn")
	src, err := resolve(t, revB)
	if err != nil {
		t.Fatal(err)
	}
	if localOf(src.NetlistURI) != revB || localOf(src.BoardURI) != revB {
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
	src, err := resolve(t, sch)
	if err != nil {
		t.Fatal(err)
	}
	if localOf(src.NetlistURI) != sch {
		t.Errorf("netlist = %q, want the named companion under --as-named", localOf(src.NetlistURI))
	}
}

// TestResolveSourceNoDescriptorIsUnchanged: every invocation that works today keeps working, and the
// .eds sibling advice survives where there is no declaration to act on.
func TestResolveSourceNoDescriptorIsUnchanged(t *testing.T) {
	dir := t.TempDir()
	eds := filepath.Join(dir, "board.eds")
	write(t, eds, "x")

	src, err := resolve(t, eds)
	if err != nil {
		t.Fatal(err)
	}
	if localOf(src.NetlistURI) != eds || src.Note != "" {
		t.Fatalf("lone .eds: src = %+v, want the named file and no note", src)
	}

	write(t, filepath.Join(dir, "board.edn"), "x")
	src, err = resolve(t, eds)
	if err != nil {
		t.Fatal(err)
	}
	if localOf(src.NetlistURI) != eds {
		t.Errorf("netlist = %q: with no descriptor there is nothing to redirect to", localOf(src.NetlistURI))
	}
	if !strings.Contains(src.Note, "board.edn") || !strings.Contains(src.Note, "authoritative") {
		t.Errorf("note = %q, want the sibling advice naming board.edn", src.Note)
	}

	// Reading the .edn itself, and a lone .eds elsewhere, stay silent; the match is case-insensitive.
	src, _ = resolve(t, filepath.Join(dir, "board.edn"))
	if src.Note != "" {
		t.Errorf(".edn input: want no note, got %q", src.Note)
	}
	up := t.TempDir()
	write(t, filepath.Join(up, "B.EDS"), "x")
	write(t, filepath.Join(up, "B.EDN"), "x")
	src, _ = resolve(t, filepath.Join(up, "B.EDS"))
	if src.Note == "" {
		t.Error(".EDS with a .EDN sibling: want the advice, got none")
	}
}

// TestResolveSourceRejectsADirectoryWithNoDescriptor: the error names the descriptor it wanted, since
// handing a directory to a reader can only produce an unsupported-extension message for something
// that is not a file at all.
func TestResolveSourceRejectsADirectoryWithNoDescriptor(t *testing.T) {
	_, err := resolve(t, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), projects.DesignDescriptor) {
		t.Fatalf("error = %v, want one naming %s", err, projects.DesignDescriptor)
	}
}

// TestResolveSourceMalformedDescriptorIsAnError: an operator who wrote a design.yaml and got silently
// ignored would read the resulting default behaviour as the engine agreeing with them.
func TestResolveSourceMalformedDescriptorIsAnError(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.edn"), "x")
	write(t, filepath.Join(dir, projects.DesignDescriptor), "name: gateway\n") // no entry
	if _, err := resolve(t, filepath.Join(dir, "a.edn")); err == nil {
		t.Fatal("a descriptor missing its entry should fail the read, not be skipped")
	}
}

// TestResolveSourceMissingPathDefersToTheReader keeps the not-found message the one a user already
// knows rather than a second one invented by the resolver.
func TestResolveSourceMissingPathDefersToTheReader(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.edn")
	src, err := resolve(t, missing)
	if err != nil {
		t.Fatalf("err = %v, want the resolver to pass through", err)
	}
	if localOf(src.NetlistURI) != missing {
		t.Errorf("netlist = %q, want the named path", localOf(src.NetlistURI))
	}
}

// TestResolveSourceEntryGetsItsCompanions: naming the entry is naming the design, so the declared
// companions supply the tiers the entry cannot. Before this the same design read by two names gave
// two different drawings, because only the FOLDER form consulted companions and the entry's own
// filename fell through to the auto-layout (the .eds/.edn pairing this exists for).
func TestResolveSourceEntryGetsItsCompanions(t *testing.T) {
	dir := designFolder(t)
	entry := filepath.Join(dir, "gateway.edn")
	src, err := resolve(t, entry)
	if err != nil {
		t.Fatal(err)
	}
	if localOf(src.NetlistURI) != entry {
		t.Errorf("netlist = %q, want the named entry", localOf(src.NetlistURI))
	}
	if localOf(src.GeometryURI) != filepath.Join(dir, "gateway.kicad_sch") {
		t.Errorf("geometry = %q, want the declared schematic companion", localOf(src.GeometryURI))
	}
	if localOf(src.BoardURI) != filepath.Join(dir, "gateway.kicad_pcb") {
		t.Errorf("board = %q, want the declared board companion", localOf(src.BoardURI))
	}
}

// TestResolveSourceEntryMatchesTheFolderForm pins the property the bug broke: a design is the same
// design whether the caller names the folder or the entry inside it.
func TestResolveSourceEntryMatchesTheFolderForm(t *testing.T) {
	dir := designFolder(t)
	byFolder, err := resolve(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	byEntry, err := resolve(t, filepath.Join(dir, "gateway.edn"))
	if err != nil {
		t.Fatal(err)
	}
	if byFolder.DesignSources != byEntry.DesignSources {
		t.Errorf("folder = %+v, entry = %+v, want identical tiers", byFolder.DesignSources, byEntry.DesignSources)
	}
}

// TestResolveSourceEntryAsNamedOptsOut: --as-named means the file alone, so it suppresses the
// companion tiers on the entry as well as the companion-to-entry redirect.
func TestResolveSourceEntryAsNamedOptsOut(t *testing.T) {
	dir := designFolder(t)
	entry := filepath.Join(dir, "gateway.edn")

	readAsNamed = true
	t.Cleanup(func() { readAsNamed = false })
	src, err := resolve(t, entry)
	if err != nil {
		t.Fatal(err)
	}
	if localOf(src.GeometryURI) != entry || localOf(src.BoardURI) != entry {
		t.Errorf("src = %+v, want every tier on the named file under --as-named", src)
	}
	if src.Note != "" {
		t.Errorf("note = %q, want none: nothing but the named file was read", src.Note)
	}
}

// TestResolveSourceEntryNoteNamesOnlyTheExtras: the caller got the file they asked for, so the note
// must not claim a redirect happened; it reports only the companions attached alongside it.
func TestResolveSourceEntryNoteNamesOnlyTheExtras(t *testing.T) {
	dir := designFolder(t)
	src, err := resolve(t, filepath.Join(dir, "gateway.edn"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(src.Note, "companion view") || strings.Contains(src.Note, "analysis reads") {
		t.Errorf("note = %q, want no redirect language: the entry was read as named", src.Note)
	}
	if !strings.Contains(src.Note, "gateway.kicad_sch") || !strings.Contains(src.Note, "gateway.kicad_pcb") {
		t.Errorf("note = %q, want it to name both companions that were read", src.Note)
	}
}

// TestResolveSourceEntryWithNoCompanionsIsSilent: a design that declares none is the ordinary case,
// and it must not gain a note saying nothing.
func TestResolveSourceEntryWithNoCompanionsIsSilent(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "plain.edn")
	write(t, entry, "x")
	write(t, filepath.Join(dir, projects.DesignDescriptor), "name: plain\nentry: plain.edn\n")
	src, err := resolve(t, entry)
	if err != nil {
		t.Fatal(err)
	}
	if localOf(src.NetlistURI) != entry || localOf(src.GeometryURI) != entry {
		t.Fatalf("src = %+v, want every tier on the entry", src)
	}
	if src.Note != "" {
		t.Errorf("note = %q, want none", src.Note)
	}
}
