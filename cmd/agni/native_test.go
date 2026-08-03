package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// runNative executes the native command tree with args, capturing stdout.
func runNative(t *testing.T, args ...string) (string, error) {
	t.Helper()
	c := nativeCmd()
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&out)
	c.SetArgs(args)
	err := c.Execute()
	return out.String(), err
}

// TestNativeOpenPrint: `native open --print` resolves the launch command without running it.
func TestNativeOpenPrint(t *testing.T) {
	pcb := filepath.Join("..", "..", "readers", "kicad", "testdata", "board.kicad_pcb")
	out, err := runNative(t, "open", "--print", pcb)
	if err != nil {
		t.Fatalf("native open --print: %v", err)
	}
	// The launcher token is platform-specific ("open" on macOS, "kicad" on Linux); assert it
	// is a known KiCad launcher and that the file is in the command.
	fields := strings.Fields(out)
	if len(fields) == 0 || (fields[0] != "open" && fields[0] != "kicad") {
		t.Errorf("open --print launcher = %q, want open (macOS) or kicad (Linux)", out)
	}
	if !strings.Contains(out, "board.kicad_pcb") {
		t.Errorf("open --print output = %q, want the fixture path", out)
	}
}

// TestNativeUnsupportedFormat: a format with no native tool produces an actionable error, not a
// crash, for both subcommands.
func TestNativeUnsupportedFormat(t *testing.T) {
	edn := filepath.Join("..", "..", "readers", "edif", "testdata", "basic.edn")
	for _, sub := range []string{"render", "open"} {
		_, err := runNative(t, sub, edn)
		if err == nil || !strings.Contains(err.Error(), "no native tool for .edn") {
			t.Errorf("native %s .edn err = %v, want a 'no native tool for .edn' message", sub, err)
		}
	}
}
