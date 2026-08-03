package native

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestOpenArgs: the GUI launch command dispatches by format (KiCad file types, and .sch
// sniffed for xschem vs Lepton) and by platform (macOS via `open -a`, otherwise the binary
// direct); formats with no native GUI report ErrNoTool.
func TestOpenArgs(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	pcb := write("b.kicad_pcb", "(kicad_pcb)")
	xsch := write("x.sch", "v {xschem version=3.4.4}\n")
	gsch := write("g.sch", "v 20200319 2\n")
	edn := write("d.edn", "(edif x)")

	bin, args, err := OpenArgs(pcb)
	if err != nil {
		t.Fatalf("OpenArgs(.kicad_pcb) err = %v", err)
	}
	switch runtime.GOOS {
	case "darwin":
		if bin != "open" || strings.Join(args, " ") != "-a KiCad "+pcb {
			t.Errorf("darwin kicad open = %q %v, want open -a KiCad %s", bin, args, pcb)
		}
	case "linux":
		if bin != "kicad" || len(args) != 1 || args[0] != pcb {
			t.Errorf("linux kicad open = %q %v, want kicad %s", bin, args, pcb)
		}
	}

	// xschem/Lepton launch the binary directly on every supported platform.
	if bin, _, err := OpenArgs(xsch); err != nil || bin != "xschem" {
		t.Errorf("OpenArgs(xschem .sch) = %q, %v, want xschem", bin, err)
	}
	if bin, _, err := OpenArgs(gsch); err != nil || bin != "lepton-schematic" {
		t.Errorf("OpenArgs(geda .sch) = %q, %v, want lepton-schematic", bin, err)
	}

	// EDIF has no native GUI.
	if _, _, err := OpenArgs(edn); !errors.Is(err, ErrNoTool) {
		t.Errorf("OpenArgs(.edn) err = %v, want ErrNoTool", err)
	}
}

// TestRenderFileGates: the CLI render skips the operator allowlist but still reports ErrNoTool
// for a format with no renderer, without needing any binary installed.
func TestRenderFileGates(t *testing.T) {
	dir := t.TempDir()
	edn := filepath.Join(dir, "d.edn")
	if err := os.WriteFile(edn, []byte("(edif x)"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RenderFile(context.Background(), edn, 1); !errors.Is(err, ErrNoTool) {
		t.Errorf("RenderFile(.edn) err = %v, want ErrNoTool", err)
	}
}

// TestRenderFileKicad exercises the real kicad-cli path when it is installed (matching the
// existing kicadInstalled gate), proving the gate-free CLI render produces an SVG.
func TestRenderFileKicad(t *testing.T) {
	if !kicadInstalled() {
		t.Skip("kicad-cli not installed")
	}
	svg, err := RenderFile(context.Background(), kicadFixtureAbs(t), 1)
	if err != nil {
		t.Fatalf("RenderFile kicad_pcb: %v", err)
	}
	if !strings.Contains(svg, "<svg") {
		t.Errorf("native render is not SVG: %.40q", svg)
	}
}

func kicadFixtureAbs(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "readers", "kicad", "testdata", "board.kicad_pcb"))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}
