package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// readDesignForTest reads through the CLI's own readDesign, so these tests exercise the same reader
// dispatch and project resolution a user gets rather than a bare loader.
func readDesignForTest(t *testing.T, path string) *ir.Design {
	t.Helper()
	d, err := readDesign(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return d
}

// TestEmitWriterDispatch pins how a format is chosen, which is the part of `emit` a user hits without
// reading the flag: the extension decides, so converting to an EDIF netlist needs no flag, and the
// pre-EDIF behavior of writing IPC-2581 survives every invocation that did not name an extension.
func TestEmitWriterDispatch(t *testing.T) {
	// The writers are compared by what they produce on an empty design rather than by function
	// identity, which Go does not allow.
	for _, tc := range []struct {
		name, format, out string
		wantEDIF, wantErr bool
		errHas            string
	}{
		{name: "edn extension", out: "board.edn", wantEDIF: true},
		{name: "edf extension", out: "board.edf", wantEDIF: true},
		{name: "uppercase extension", out: "BOARD.EDIF", wantEDIF: true},
		{name: "stdout has no extension", out: ""},
		{name: "xml stays ipc2581", out: "board.xml"},
		{name: "flag beats extension", format: "ipc2581", out: "board.edn"},
		{name: "flag beats stdout default", format: "edif", out: "", wantEDIF: true},
		{name: "eds is refused", out: "board.eds", wantErr: true, errHas: "only the netlist writer exists"},
		{name: "eds yields to an explicit flag", format: "edif", out: "board.eds", wantEDIF: true},
		{name: "unknown format", format: "gerber", wantErr: true, errHas: "unknown emit format"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, err := emitWriter(tc.format, tc.out)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("emitWriter(%q, %q) = no error, want one", tc.format, tc.out)
				}
				if !strings.Contains(err.Error(), tc.errHas) {
					t.Errorf("error %q should mention %q", err, tc.errHas)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if err := w(&buf, readDesignForTest(t, "testdata/conformance/crystal.passes.edn")); err != nil {
				t.Fatal(err)
			}
			if gotEDIF := strings.HasPrefix(buf.String(), "(edif "); gotEDIF != tc.wantEDIF {
				t.Errorf("wrote EDIF = %v, want %v; output starts %.40q", gotEDIF, tc.wantEDIF, buf.String())
			}
		})
	}
}

// TestEmitEDIFRoundTripsThroughTheCLI is the end-to-end the unit test above cannot be: the command
// reads a design, writes it, and reads the result back with the same reader dispatch a user would
// get. readers/edif proves the writer preserves the IR; this proves the CLI reaches it at all.
func TestEmitEDIFRoundTripsThroughTheCLI(t *testing.T) {
	src := filepath.Join("testdata", "conformance", "showcase.fires.kicad_sch")
	out := filepath.Join(t.TempDir(), "showcase.edn")

	cmd := rootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"emit", src, out})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("emit: %v\n%s", err, buf.String())
	}

	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), "(edif ") {
		t.Fatalf("the .edn extension should have selected EDIF, got %.60q", raw)
	}

	// A KiCad schematic carries no EDIF root refs, so this also exercises the writer's minted top
	// cell and work library -- the path the round-trip oracle in readers/edif never reaches, because
	// every fixture there came from EDIF in the first place.
	from, to := readDesignForTest(t, src), readDesignForTest(t, out)
	if len(from.GetComponents()) == 0 {
		t.Fatal("fixture has no components; the comparison below would pass vacuously")
	}
	if got, want := len(to.GetComponents()), len(from.GetComponents()); got != want {
		t.Errorf("components after the round trip = %d, want %d", got, want)
	}
	if got, want := len(to.GetNets()), len(from.GetNets()); got != want {
		t.Errorf("nets after the round trip = %d, want %d", got, want)
	}
}
