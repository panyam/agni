package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/agni/project"
	"github.com/panyam/agni/readers/formats"
)

// readAsNamed disables descriptor-driven redirection: the analysis reads exactly the file named, even
// when the enclosing design declares it a companion view of a different entry. Set by --as-named.
//
// It exists because reading a companion AS a netlist is a legitimate DIAGNOSTIC operation, not only a
// mistake. Checking that a schematic export and the netlist still describe the same design means
// reading both as netlists and diffing them, which is exactly what the redirect would otherwise
// prevent (`examples/tutorial-project` does this in its `check-views` target).
var readAsNamed bool

// designSource is which file each tier of a read comes from, once the enclosing design's descriptor
// has had its say (agni issue 170).
//
// The engine used to take the path a user typed as the whole answer, which is why `warnEdsSibling`
// existed: a folder holding an OrCAD `.eds` schematic export beside the `.edn` netlist reads a
// different component count depending on which one a tool opens, and with nothing modelling "this
// folder is one design, and this file is its entry", the CLI could only print a warning telling the
// operator to go consult their own descriptor. A declared entry turns that advice into behaviour.
type designSource struct {
	// Netlist is the file component and connectivity ANALYSIS reads: the netlist the design team
	// produces (CONSTRAINTS C21).
	Netlist string
	// Board is the file the board tier's copper geometry comes from. It is often the same file, and
	// differs when a design declares a separate board companion.
	Board string
	// Geometry is the file schematic geometry is rendered and located from. A netlist entry carries
	// none of its own, so a design that declares a schematic companion locates its findings on that
	// companion's sheets — which is what C21 means by a companion being a canvas rather than a source.
	Geometry string
	// Note is a line for stderr, empty when the named path was taken exactly as given. It is written
	// whenever a file the user did NOT name was read, because which file was read is not recoverable
	// from a component count or a findings list — the same reason the viewer's convention bar names
	// the active vocabulary instead of only offering a dropdown.
	Note string
}

// resolveSource decides which files a read should actually open, given the path a user named.
//
// Three cases, and only the first two involve a descriptor:
//
//   - A DESIGN FOLDER (one holding `design.yaml`) reads its declared entry, and picks up a declared
//     board companion for the board tier. This is new capability: pointing agni at a design used to
//     be an error, so there was no way to say "this design" rather than "this file".
//   - A file the enclosing design declared a COMPANION reads the design's entry instead, with the
//     named file supplying board geometry when it carries any. Redirection is confined to files an
//     operator explicitly listed as companions rather than to "anything beside the entry", because a
//     later revision of the netlist sits in the same folder and IS a legitimate analysis source; an
//     inferred rule would turn a diff of two revisions into a diff of one against itself.
//   - Anything else reads exactly what was named, which is every invocation that works today.
//
// A malformed descriptor is an error rather than a fallback to the plain read. An operator who wrote
// a design.yaml and got silently ignored would read the resulting behaviour as the engine agreeing
// with them.
func resolveSource(path string) (designSource, error) {
	plain := designSource{Netlist: path, Board: path, Geometry: path}
	fi, err := os.Stat(path)
	if err != nil {
		// Let the reader produce the not-found error, so the message a user sees is the one they
		// already know rather than a new one from the resolver.
		return plain, nil
	}
	if fi.IsDir() {
		return resolveDesignDir(path)
	}
	if readAsNamed {
		return plain, nil
	}
	dir := filepath.Dir(path)
	d, found, err := designAt(dir)
	if err != nil {
		return designSource{}, err
	}
	if !found || !d.IsCompanion(filepath.Base(path)) {
		plain.Note = edsSiblingNote(path)
		return plain, nil
	}
	entry := filepath.Join(dir, filepath.FromSlash(project.CleanRel(d.Entry)))
	src := designSource{Netlist: entry, Board: entry, Geometry: entry}
	// The named companion keeps whatever tier it alone can supply. A board file's copper and a
	// schematic export's sheets are the reason the operator pointed at it; only the netlist moves.
	if formats.HasBoard(path) {
		src.Board = path
	}
	if formats.HasFaithful(path) {
		src.Geometry = path
	}
	src.Note = fmt.Sprintf("note: %s is a companion view declared by %s; analysis reads %s (the design's entry). Pass --as-named to read the file itself.\n",
		filepath.Base(path), filepath.Join(dir, project.DesignDescriptor), filepath.Base(entry))
	return src, nil
}

// resolveDesignDir resolves a directory the user named. A directory that declares no design is an
// error naming the descriptor it wanted, because the alternative is handing the path to a reader that
// can only report an unsupported extension for something that is not a file at all.
func resolveDesignDir(dir string) (designSource, error) {
	d, found, err := designAt(dir)
	if err != nil {
		return designSource{}, err
	}
	if !found {
		return designSource{}, fmt.Errorf("%s is a directory with no %s: name a design file, or add a descriptor declaring which file is this design's entry", dir, project.DesignDescriptor)
	}
	entry := filepath.Join(dir, filepath.FromSlash(project.CleanRel(d.Entry)))
	src := designSource{Netlist: entry, Board: entry, Geometry: entry}
	// The board and geometry tiers come from declared companions. A netlist entry carries neither of
	// its own, so without this a design whose descriptor names the board file sitting right beside it
	// would still report every board-tier rule not-applicable.
	var from []string
	for _, c := range d.Companions {
		cp := filepath.Join(dir, filepath.FromSlash(project.CleanRel(c)))
		if src.Board == entry && formats.HasBoard(cp) {
			src.Board = cp
			from = append(from, "board geometry from "+filepath.Base(cp))
		}
		if src.Geometry == entry && formats.HasFaithful(cp) {
			src.Geometry = cp
			from = append(from, "sheets from "+filepath.Base(cp))
		}
	}
	src.Note = fmt.Sprintf("note: reading %s (the entry %s declares)", entry, filepath.Join(dir, project.DesignDescriptor))
	if len(from) > 0 {
		src.Note += ", with " + strings.Join(from, " and ")
	}
	src.Note += ".\n"
	return src, nil
}

// designAt loads the design descriptor in dir, reporting whether there was one.
//
// It reads the ONE directory it was handed rather than walking upward. The CLI is pointed at a path
// by a human, and a descriptor two folders up claiming a file the operator named directly would be
// action at a distance; the serve side, which addresses resources rather than paths, is where the
// upward walk belongs (project.Tree.Resolve).
//
// A malformed descriptor is an error rather than a skip: an operator who wrote a design.yaml and got
// silently ignored would read the resulting default behaviour as the engine agreeing with them.
func designAt(dir string) (project.Design, bool, error) {
	f, err := os.Open(filepath.Join(dir, project.DesignDescriptor))
	if err != nil {
		return project.Design{}, false, nil
	}
	defer f.Close()
	d, err := project.LoadDesign(f)
	if err != nil {
		return project.Design{}, false, fmt.Errorf("%s: %w", filepath.Join(dir, project.DesignDescriptor), err)
	}
	return d, true, nil
}

// edsSiblingNote is the fallback advice for a folder that declares no design: reading an EDIF
// SCHEMATIC-geometry (.eds) export while the sibling NETLIST (.edn) exists. The two read different
// component counts — the .eds reflects what the schematic DRAWS, the .edn is authoritative for
// component identity — so a count taken off the .eds is a silent footgun (it read test_point 565
// against the .edn's 1385 on one real board). Empty when there is no .eds, no sibling .edn, or the
// input already is the .edn.
//
// It survives as ADVICE only where there is no declaration to act on. A design folder that names its
// entry gets the redirect above instead, which is the same guardrail with the guess removed: it
// names the file the operator actually declared, and it fires for a `.kicad_pcb` or an IPC-2581
// companion just as readily as for a `.eds`, where this heuristic only ever knew one stem-matched
// pair.
func edsSiblingNote(path string) string {
	if !strings.EqualFold(filepath.Ext(path), ".eds") {
		return ""
	}
	stem := strings.TrimSuffix(path, filepath.Ext(path))
	for _, ext := range []string{".edn", ".EDN"} {
		if _, err := os.Stat(stem + ext); err == nil {
			return fmt.Sprintf("warning: %s is an EDIF schematic-geometry (.eds) export; component counts may be lower than the netlist. The sibling %s (.edn netlist) is authoritative for component identity — declare it as this design's entry in a %s.\n",
				filepath.Base(path), filepath.Base(stem+ext), project.DesignDescriptor)
		}
	}
	return ""
}
