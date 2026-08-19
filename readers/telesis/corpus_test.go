package telesis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// CorpusEnv names the directory of real Telesis exports this test reads.
const CorpusEnv = "AGNI_TELESIS_CORPUS"

// TestCorpus runs the reader over real exports, and is skipped unless CorpusEnv points at a
// directory holding some.
//
// WHY THIS EXISTS AS A SEPARATE, SKIPPED TEST. Everything this reader knows about the format came
// from reading real files, and the committed fixtures were then written from that same
// understanding. Validating the parser only against those fixtures is circular: a misreading of the
// grammar produces a fixture that encodes the misreading and a parser that agrees with it, and both
// halves pass. Real files are the only thing that breaks the circle.
//
// They cannot be committed. A netlist carries net names, reference designators and part numbers off
// a real design, so no export, and no fragment of one, belongs in this repository (see the root
// CLAUDE.md). Hence the split: synthetic fixtures in testdata/ that CI runs, and this, which only
// someone holding real exports can run.
//
// The assertions are deliberately STRUCTURAL rather than value-based. Nothing here names a net, a
// designator or a part, so a failure message cannot leak design content into a terminal, a log or a
// CI artifact. What it checks is that the reader's model of the format survives contact with files
// it was not written against: sections resolve, the two property scopes separate, directions
// resolve for a real fraction of pins, and nothing lands in the bucket that means "I did not
// understand this".
func TestCorpus(t *testing.T) {
	dir := os.Getenv(CorpusEnv)
	if dir == "" {
		t.Skipf("set %s to a directory of real .tel exports to run this", CorpusEnv)
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*.tel"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatalf("%s=%s holds no .tel files", CorpusEnv, dir)
	}

	for _, path := range matches {
		// The file NAME can carry a project or board name, so the subtest is numbered rather than
		// named after it, for the same reason the assertions below are structural.
		t.Run(anonymize(path), func(t *testing.T) {
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			d, err := Read(f, path)
			if err != nil {
				t.Fatalf("read failed: %v", err)
			}

			if len(d.Components) == 0 {
				t.Error("no components; $PACKAGES did not resolve")
			}
			if len(d.Nets) == 0 {
				t.Error("no nets; $NETS did not resolve")
			}

			// Every connection must name a component the packages declared. A connection pointing
			// at nothing means the two sections were read with different notions of a ref-des.
			known := map[string]bool{}
			for _, c := range d.Components {
				known[c.RefDes] = true
			}
			var orphanConns int
			for _, n := range d.Nets {
				for _, c := range n.Connections {
					if !known[c.ComponentRef] {
						orphanConns++
					}
				}
			}
			if orphanConns > 0 {
				t.Errorf("%d connections name a component no package declared", orphanConns)
			}

			// Component attributes must attach to real components, and none may be a pin fact.
			// A `Pin Type` on a component means the target-shape routing let a pin target through
			// as component-scoped, which is the failure this reader's shape discrimination exists
			// to prevent.
			var pinFactsOnComponents int
			for _, c := range d.Components {
				if _, bad := c.GetAttributes()[PinTypeProperty]; bad {
					pinFactsOnComponents++
				}
			}
			if pinFactsOnComponents > 0 {
				t.Errorf("%d components carry a pin-scoped property; scope routing is wrong", pinFactsOnComponents)
			}

			// Direction must resolve for most pins. A real export types nearly every pin, so a low
			// rate means the pin property block was misread rather than that the design is unusual.
			typed, unknown, total := 0, 0, 0
			for _, l := range d.Libraries {
				for _, p := range l.Parts {
					for _, pin := range p.Pins {
						total++
						if pin.Direction != ir.PinDirection_PIN_DIRECTION_UNSPECIFIED {
							typed++
						}
						if pin.GetAttributes()["direction_raw"] != "" && pin.Direction == ir.PinDirection_PIN_DIRECTION_UNSPECIFIED {
							unknown++
						}
					}
				}
			}
			if total == 0 {
				t.Fatal("no pins resolved; the pin property block did not read")
			}
			if pct := typed * 100 / total; pct < 80 {
				t.Errorf("only %d%% of %d pins have a direction, want >=80%%", pct, total)
			}
			// An unmapped Pin Type is not a failure, it is news: it means this export uses a value
			// the mapping has never seen, and the vocabulary needs widening.
			if unknown > 0 {
				t.Logf("%d pins carry a Pin Type this reader does not map; the vocabulary needs widening", unknown)
			}

			// Not a failure: a section this reader does not consume does not make what it DID
			// read wrong. But it is the first thing anyone running this against a new exporter
			// needs to know, and on the exports this reader was built against it stays empty.
			if u := d.GetAttributes()[UnparsedSectionsAttr]; u != "" {
				t.Logf("sections carrying content this reader did not consume: %s", u)
			}
			t.Logf("structure: %d components, %d nets, %d part types, %d pins, %d%% typed",
				len(d.Components), len(d.Nets), countParts(d), total, typed*100/total)
		})
	}
}

func countParts(d *ir.Design) int {
	n := 0
	for _, l := range d.Libraries {
		n += len(l.Parts)
	}
	return n
}

// anonymize reduces a corpus path to a subtest name that cannot carry a project or board name,
// since a subtest name is printed on every run and ends up in CI logs.
func anonymize(path string) string {
	base := filepath.Base(path)
	return strings.Repeat("x", len(strings.TrimSuffix(base, ".tel"))) + ".tel"
}
