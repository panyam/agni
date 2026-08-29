package edif

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/panyam/agni/core/diff"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// corpus is every EDIF netlist this repo commits, in three groups: the reader's own grammar fixtures
// (names, buses, hierarchy, duplicate ports, escaped ids, wrapped tokens), the CLI conformance
// designs, and the tutorial board, which is the largest and the only one written to be read by a
// person. A directory that is not there is skipped rather than failed, so moving one of the other
// two groups does not break this package.
func corpus(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, dir := range []string{
		"testdata",
		filepath.Join("..", "..", "cmd", "agni", "testdata", "conformance"),
		filepath.Join("..", "..", "examples", "tutorial-project", "designs", "gateway"),
	} {
		if _, err := os.Stat(dir); err != nil {
			t.Logf("skipping %s: %v", dir, err)
			continue
		}
		found, err := filepath.Glob(filepath.Join(dir, "*.edn"))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, found...)
	}
	if len(out) == 0 {
		t.Fatal("no .edn fixtures found; the oracle would pass vacuously")
	}
	return out
}

func readFile(t *testing.T, path string) *ir.Design {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	d, err := Read(bytes.NewReader(raw), path)
	if err != nil {
		t.Fatalf("%s: read: %v", path, err)
	}
	return d
}

// roundTrip writes a design and reads the result back, which is the operation the oracle is about.
func roundTrip(t *testing.T, d *ir.Design, path string) *ir.Design {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteNetlist(&buf, d); err != nil {
		t.Fatalf("%s: write: %v", path, err)
	}
	got, err := Read(&buf, path)
	if err != nil {
		t.Fatalf("%s: re-read of emitted netlist: %v\n%s", path, err, buf.String())
	}
	return got
}

// TestWriteRoundTripsIR is the C6 oracle for the netlist writer: read, write, read, and require the
// two IRs to match. C6 obliges only a LOSSLESS reader to round-trip byte-for-byte, and this one
// declares lossy-bounded, so the assertion is at the IR level -- the level every consumer of this
// package actually reads.
//
// Three exclusions, each because the READER already dropped the information before the writer could
// see it. They are listed here rather than quietly normalized away, because the next person to hit
// one needs to know it is the reader's loss being reported and not a writer bug to go and fix:
//
//   - edif_hierarchical. extract scopes to the design's top cell and drops every sub-cell's
//     contents, so writing a hierarchical design emits a flat one and the flag, which is computed
//     from the count of cells carrying instances, is absent on the re-read. Making this pass would
//     mean emitting a sub-cell whose contents no longer exist, which is fabricating structure.
//   - Provenance.SourceFile. Different by construction; the writer takes a design, not a path.
//   - InputDiagnostics.UnmodeledBuses, and with them the NAMELESS PINS an array port leaves behind.
//     A BusNotModeled records the bus label and its members but not the cell or port it was declared
//     on, so an array port has nowhere to be written back to; and partTypeOf, whose parseName knows
//     four name forms and not (array DATA 8), files the port itself as a pin with no name at all.
//     Excluding one without the other would assert that a bus we decline to write comes back anyway.
func TestWriteRoundTripsIR(t *testing.T) {
	for _, path := range corpus(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			want := readFile(t, path)
			got := roundTrip(t, want, path)
			if diffIR(t, want, got); t.Failed() {
				var buf bytes.Buffer
				_ = WriteNetlist(&buf, want)
				t.Logf("emitted netlist:\n%s", buf.String())
			}
		})
	}
}

// diffIR compares two designs field by field, reporting the first difference per section rather than
// dumping two protos. A whole-message proto.Equal says only "not equal", which on a design with
// nineteen components and fifteen nets is not a lead.
func diffIR(t *testing.T, want, got *ir.Design) {
	t.Helper()
	w, g := normalize(want), normalize(got)
	if w.GetName() != g.GetName() {
		t.Errorf("design name = %q, want %q", g.GetName(), w.GetName())
	}
	if !reflect.DeepEqual(w.GetAttributes(), g.GetAttributes()) {
		t.Errorf("attributes = %v, want %v", g.GetAttributes(), w.GetAttributes())
	}
	if lw, lg := len(w.GetLibraries()), len(g.GetLibraries()); lw != lg {
		t.Errorf("libraries = %d, want %d", lg, lw)
	}
	if cw, cg := len(w.GetComponents()), len(g.GetComponents()); cw != cg {
		t.Errorf("components = %d, want %d", cg, cw)
	}
	if nw, ng := len(w.GetNets()), len(g.GetNets()); nw != ng {
		t.Errorf("nets = %d, want %d", ng, nw)
	}
	if t.Failed() {
		return
	}
	for i, lw := range w.GetLibraries() {
		if lg := g.GetLibraries()[i]; !proto.Equal(lw, lg) {
			t.Errorf("library %d:\n got %v\nwant %v", i, lg, lw)
		}
	}
	for i, cw := range w.GetComponents() {
		if cg := g.GetComponents()[i]; !proto.Equal(cw, cg) {
			t.Errorf("component %d (%s):\n got %v\nwant %v", i, cw.GetRefDes(), cg, cw)
		}
	}
	for i, nw := range w.GetNets() {
		if ng := g.GetNets()[i]; !proto.Equal(nw, ng) {
			t.Errorf("net %d (%s):\n got %v\nwant %v", i, nw.GetName(), ng, nw)
		}
	}
	if !proto.Equal(w.GetInputDiagnostics(), g.GetInputDiagnostics()) {
		t.Errorf("input diagnostics:\n got %v\nwant %v", g.GetInputDiagnostics(), w.GetInputDiagnostics())
	}
}

// normalize applies the three documented exclusions to a copy, so neither input is mutated.
func normalize(d *ir.Design) *ir.Design {
	c := proto.Clone(d).(*ir.Design)
	clearSourceFiles(c.ProtoReflect())
	delete(c.Attributes, "edif_hierarchical")
	for _, lib := range c.GetLibraries() {
		for _, pt := range lib.GetParts() {
			kept := pt.Pins[:0]
			for _, p := range pt.GetPins() {
				if p.GetName() != "" {
					kept = append(kept, p)
				}
			}
			pt.Pins = kept
		}
	}
	if id := c.GetInputDiagnostics(); id != nil {
		id.UnmodeledBuses = nil
		// A diagnostics message holding nothing but the excluded buses must compare equal to the
		// absent one the re-read produces, not to an empty struct.
		if len(id.GetUnannotatedComponents()) == 0 {
			c.InputDiagnostics = nil
		}
	}
	return c
}

// clearSourceFiles walks the message tree and clears every Provenance.source_file, the same
// reflective shape relocateSources uses on the read side rather than a hand-listed set of paths that
// would go stale the moment a message gains a Provenance.
func clearSourceFiles(m protoreflect.Message) {
	if fd := m.Descriptor().Fields().ByName("source_file"); fd != nil {
		m.Clear(fd)
	}
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.IsMap():
			if fd.MapValue().Kind() == protoreflect.MessageKind {
				v.Map().Range(func(_ protoreflect.MapKey, mv protoreflect.Value) bool {
					clearSourceFiles(mv.Message())
					return true
				})
			}
		case fd.IsList():
			if fd.Kind() == protoreflect.MessageKind {
				l := v.List()
				for i := range l.Len() {
					clearSourceFiles(l.Get(i).Message())
				}
			}
		case fd.Kind() == protoreflect.MessageKind:
			clearSourceFiles(v.Message())
		}
		return true
	})
}

// TestWritePreservesDiffIdentity is the second-order check the first-order one cannot make. An IR
// comparison is per design; a diff is BETWEEN two, and it keys on identity that survives a
// revision (net names, ref-des, section part refs) while deliberately ignoring the export-unstable
// internal ids. A writer could therefore perturb exactly the fields diff keys on and still pass
// TestWriteRoundTripsIR if it perturbed them the same way on both sides.
//
// So: the report between the two committed revisions must equal the report between their
// round-tripped forms. rev_a and rev_b carry the whole change taxonomy already
// (TestEDIFDiffRoundTrip pins it), which is what makes them the right pair.
func TestWritePreservesDiffIdentity(t *testing.T) {
	a, b := readFile(t, filepath.Join("testdata", "rev_a.edn")), readFile(t, filepath.Join("testdata", "rev_b.edn"))
	want := diff.Designs(a, b)
	// The same path strings the originals were read with: provenance carries the source file, diff
	// carries provenance into its report, so a round trip read under a different name would differ here
	// for a reason that has nothing to do with the writer.
	got := diff.Designs(
		roundTrip(t, a, filepath.Join("testdata", "rev_a.edn")),
		roundTrip(t, b, filepath.Join("testdata", "rev_b.edn")))
	if !reflect.DeepEqual(want, got) {
		t.Errorf("diff over round-tripped designs differs\n got %+v\nwant %+v", got, want)
	}
}
