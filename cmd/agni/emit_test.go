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

// TestEmitFormatDispatch pins how a format is chosen, which is the part of `emit` a user hits without
// reading the flag: the extension decides, so converting to an EDIF netlist needs no flag, and the
// pre-EDIF behavior of writing IPC-2581 survives every invocation that did not name an extension.
func TestEmitFormatDispatch(t *testing.T) {
	for _, tc := range []struct {
		name, format, out string
		want              string
		errHas            string
	}{
		{name: "edn extension", out: "board.edn", want: emitEDIF},
		{name: "edf extension", out: "board.edf", want: emitEDIF},
		{name: "uppercase extension", out: "BOARD.EDIF", want: emitEDIF},
		{name: "stdout has no extension", out: "", want: emitIPC2581},
		{name: "xml stays ipc2581", out: "board.xml", want: emitIPC2581},
		{name: "flag beats extension", format: "ipc2581", out: "board.edn", want: emitIPC2581},
		{name: "flag beats stdout default", format: "edif", out: "", want: emitEDIF},
		{name: "flag case is not the user's problem", format: "EDIF", out: "", want: emitEDIF},
		{name: "eds is refused", out: "board.eds", errHas: "only the netlist writer exists"},
		{name: "eds yields to an explicit flag", format: "edif", out: "board.eds", want: emitEDIF},
		{name: "unknown format", format: "gerber", errHas: "unknown emit format"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := emitFormat(tc.format, tc.out)
			if tc.errHas != "" {
				if err == nil {
					t.Fatalf("emitFormat(%q, %q) = %q, want an error", tc.format, tc.out, got)
				}
				if !strings.Contains(err.Error(), tc.errHas) {
					t.Errorf("error %q should mention %q", err, tc.errHas)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("emitFormat(%q, %q) = %q, want %q", tc.format, tc.out, got, tc.want)
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
