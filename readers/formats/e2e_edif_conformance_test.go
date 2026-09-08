package formats

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/panyam/agni/readers/edif"
)

// The three properties below are what separates EDIF a foreign reader can use from EDIF only our own
// reader can. Every one of them was false for every file this writer produced before agni issue 580,
// and none was visible to the round-trip oracle, because our reader is forgiving in exactly the three
// places a conforming one is not: it resolves references after the whole file is parsed rather than
// as it goes, it accepts any atom the tokenizer returns as an identifier, and it falls back to
// treating a port reference as a pin designator when no portInstance table maps it.
//
// They are asserted over the emitted TEXT rather than over a re-read, deliberately. A re-read goes
// back through the same forgiving reader and so agrees with the writer no matter what either does,
// which is the whole reason this went unnoticed. GNU Electric is the out-of-tree check that these
// three are the RIGHT properties (see the issue for how to run it); these tests are what keeps them
// true without a 23MB Java dependency in the gate.

// TestEmitEDIFDeclaresBeforeReferencing pins the file ORDER. A reader that resolves a cellRef when it
// meets it, rather than after parsing the whole document, cannot follow a forward reference, and the
// design node's cellRef names the top cell. We used to write that node second, ahead of every
// library, so the very first reference in the file pointed at a cell that did not exist yet.
func TestEmitEDIFDeclaresBeforeReferencing(t *testing.T) {
	for _, tc := range emitCases {
		t.Run(tc.name, func(t *testing.T) {
			text := emitText(t, tc.path)
			design := strings.Index(text, "\n  (design ")
			if design < 0 {
				t.Fatal("no design node found; the assertion below would pass vacuously")
			}
			if lib := strings.LastIndex(text, "\n  (library "); lib > design {
				t.Errorf("a (library ...) starts at byte %d, after the design node at %d; "+
					"the design's cellRef is then a forward reference", lib, design)
			}
		})
	}
}

// badAtom is the character set GNU Electric refuses in a cell name, whitespace aside. Meeting one
// aborts its whole import rather than skipping the cell, so a single KiCad part named `lib:Part`
// took every design read from KiCad with it.
var badAtom = regexp.MustCompile(`[:;{}|]`)

// identifier matches the identifier position of every construct that declares or references one. The
// display half of a (rename ID "Display") is deliberately not checked: it is a quoted string, so it
// can hold anything, and it is where a name too rich for the grammar is supposed to end up.
var identifier = regexp.MustCompile(`\((?:cell|library|instance|port|portInstance|net|cellRef|libraryRef|instanceRef|portRef|viewRef|rename) ([^\s()"]+)`)

// TestEmitEDIFWritesLegalIdentifiers pins that no identifier carries a character a reader rejects.
// The escape hatch is the rename, whose quoted display half is unconstrained, so nothing is lost by
// holding the identifier half to the grammar.
//
// It does NOT make every file Electric will load, and the difference is worth knowing before reading
// a green run as one. Electric takes a cell's rename DISPLAY as the cell's name (EDIF.java, KeyRename)
// where our reader takes the identifier, so a KiCad part named `gateway:CAP` still stops its import
// even though the identifier beside it is clean. Which of the two a rename really names is a genuine
// disagreement between readers, and picking a different display would mean discarding the part name
// the source gave. Tracked on agni issue 580 rather than papered over here.
func TestEmitEDIFWritesLegalIdentifiers(t *testing.T) {
	for _, tc := range emitCases {
		t.Run(tc.name, func(t *testing.T) {
			var bad []string
			for _, m := range identifier.FindAllStringSubmatch(emitText(t, tc.path), -1) {
				if badAtom.MatchString(m[1]) {
					bad = append(bad, m[0])
				}
			}
			if len(bad) > 0 {
				sort.Strings(bad)
				t.Errorf("%d identifier(s) carry a character a reader rejects, first few: %v",
					len(bad), bad[:min(5, len(bad))])
			}
		})
	}
}

// TestEmitEDIFResolvesEveryPortRef is the one that matters most, and the one the round trip could
// never see. A portRef names a PORT on the instance's cell. We named the pin's physical designator
// instead, which our reader recovers through netOf's no-mapping fallback and nobody else does, so a
// conforming reader looked for a port by that name, found none, and dropped the connection. On the
// gateway board that was 34 of 56 references, and the ones that survived did so only because a
// resistor's ports happen to be literally named "1" and "2".
//
// A reference resolves if the cell declares a port of that name, or the instance's portInstance
// table maps it. The table is what the writer now rebuilds from the part type's pins.
//
// Two rows are CHARACTERIZED rather than clean, and the count is asserted so the gap stays visible.
// A design read from a board file or from IPC-2581 carries NO part types at all, so there is no cell
// for a port to be declared on and no library holding one. Completing an interface is one thing, and
// the writer does it; minting the cell and the top cell to put it in is the open half of agni issue
// 580. When that lands these two constants go to zero.
func TestEmitEDIFResolvesEveryPortRef(t *testing.T) {
	for _, tc := range emitCases {
		t.Run(tc.name, func(t *testing.T) {
			f := scanEmitted(emitText(t, tc.path))
			if len(f.portRefs) == 0 {
				t.Fatal("no anchored portRefs; the assertion would pass vacuously")
			}
			var unresolved []string
			for _, r := range f.portRefs {
				cell, ok := f.instCell[r.inst]
				if !ok {
					unresolved = append(unresolved, fmt.Sprintf("%s: no such instance", r.inst))
					continue
				}
				if f.cellPorts[cell][r.port] || f.instPorts[r.inst][r.port] {
					continue
				}
				unresolved = append(unresolved,
					fmt.Sprintf("(portRef %s (instanceRef %s)): cell %s declares no port %s and the instance maps none",
						r.port, r.inst, cell, r.port))
			}
			sort.Strings(unresolved)
			if len(unresolved) != tc.unresolvedRefs {
				t.Errorf("%d of %d portRef(s) name nothing on the instance's cell, want %d; first few: %v",
					len(unresolved), len(f.portRefs), tc.unresolvedRefs,
					unresolved[:min(4, len(unresolved))])
			}
		})
	}
}

func emitText(t *testing.T, path string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := edif.WriteNetlist(&buf, readForEmit(t, path)); err != nil {
		t.Fatalf("%s: write: %v", path, err)
	}
	return buf.String()
}

// emitted is the slice of an EDIF file these assertions need: which cell each instance is of, which
// ports each cell declares, which ports each instance maps, and every anchored portRef.
type emitted struct {
	instCell  map[string]string
	cellPorts map[string]map[string]bool
	instPorts map[string]map[string]bool
	portRefs  []portRef
}

type portRef struct{ port, inst string }

var (
	reCell     = regexp.MustCompile(`^\s*\(cell (?:\(rename ([^\s()"]+)|([^\s()"]+))`)
	rePort     = regexp.MustCompile(`^\s*\(port (?:\(rename ([^\s()"]+)|([^\s()"]+))`)
	reInstance = regexp.MustCompile(`^\s*\(instance ([^\s()"]+) \(viewRef [^\s()"]+ \(cellRef ([^\s()"]+)`)
	rePortInst = regexp.MustCompile(`^\s*\(portInstance ([^\s()"]+)`)
	rePortRef  = regexp.MustCompile(`\(portRef ([^\s()"]+) \(instanceRef ([^\s()"]+)\)\)`)
)

// scanEmitted reads the writer's own line-oriented layout rather than parsing EDIF properly, which is
// enough because the layout is what the writer controls: one cell, port, instance or portInstance per
// line, and every portRef of a net on the net's single line.
func scanEmitted(text string) emitted {
	f := emitted{
		instCell:  map[string]string{},
		cellPorts: map[string]map[string]bool{},
		instPorts: map[string]map[string]bool{},
	}
	cell, inst := "", ""
	for line := range strings.SplitSeq(text, "\n") {
		switch {
		case reCell.MatchString(line):
			m := reCell.FindStringSubmatch(line)
			cell, inst = firstNonEmpty(m[1], m[2]), ""
			f.cellPorts[cell] = map[string]bool{}
		case reInstance.MatchString(line):
			m := reInstance.FindStringSubmatch(line)
			inst = m[1]
			f.instCell[inst] = m[2]
			f.instPorts[inst] = map[string]bool{}
		case rePortInst.MatchString(line) && inst != "":
			f.instPorts[inst][rePortInst.FindStringSubmatch(line)[1]] = true
		case rePort.MatchString(line) && cell != "":
			m := rePort.FindStringSubmatch(line)
			f.cellPorts[cell][firstNonEmpty(m[1], m[2])] = true
		}
		for _, m := range rePortRef.FindAllStringSubmatch(line, -1) {
			f.portRefs = append(f.portRefs, portRef{port: m[1], inst: m[2]})
		}
	}
	return f
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
