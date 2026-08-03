// Command convert is the emit rung of the Agni examples ladder: read any supported
// source format into the neutral IR and emit it back out, so a conversion is just
// A -> IR -> B with the IR as the pivot. Today the writer is IPC-2581, so this converts
// EDIF or KiCad (or IPC-2581 itself) into IPC-2581 and proves the semantic round-trip.
//
// The narration lives in the sidecar walkthrough.md (loaded via demokit's FromMarkdown),
// so this file only binds the steps that run engine code and wires the renderer.
//
// Run modes (see the Makefile): `make run` (plain text), `make demo` (TUI boxes),
// `make runquiet` (non-interactive defaults, CI-safe), `make doc` (render to markdown).
package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"

	"github.com/panyam/demokit"
	"github.com/panyam/agni/examples/common"
	"github.com/panyam/agni/ipc2581"
)

//go:embed walkthrough.md
var walkthroughMD []byte

func main() {
	// chosen / format carry the picks from the input steps to the emit step. They default to
	// the same values the markdown inputs default to, so a non-interactive run stays coherent.
	chosen := "demo-board.kicad_pcb"
	format := "ipc-2581"

	demo := demokit.New("convert").
		Dir("convert").
		FromMarkdownBytes(walkthroughMD)

	demo.Bind("pick").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		if v, ok := ctx.Inputs["design"].(string); ok && v != "" {
			chosen = v
		}
		fmt.Printf("Input: %s\n", chosen)
		return nil
	})

	demo.Bind("to-ir").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := common.ReadFixture(chosen)
		if err != nil {
			return demokit.Errf("read %s: %v", chosen, err)
		}
		fmt.Println(common.StatsLines(d))
		return nil
	})

	demo.Bind("pick-format").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		if v, ok := ctx.Inputs["format"].(string); ok && v != "" {
			format = v
		}
		fmt.Printf("Output: %s\n", format)
		return nil
	})

	demo.Bind("emit").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		if format != "ipc-2581" {
			return demokit.Errf("no writer for %q yet (have: ipc-2581)", format)
		}
		d, err := common.ReadFixture(chosen)
		if err != nil {
			return demokit.Errf("read %s: %v", chosen, err)
		}
		var buf bytes.Buffer
		if err := ipc2581.Write(&buf, d); err != nil {
			return demokit.Errf("emit ipc-2581: %v", err)
		}
		fmt.Printf("%s -> IR -> ipc-2581 (%d bytes):\n\n%s\n", chosen, buf.Len(), head(buf.String(), 16))

		// Read the emitted document straight back; matching stats show the modeled IR survived.
		rt, err := ipc2581.Read(bytes.NewReader(buf.Bytes()), "roundtrip.xml")
		if err != nil {
			return demokit.Errf("re-read emitted ipc-2581: %v", err)
		}
		fmt.Printf("\nround-trip re-read:\n%s\n", common.StatsLines(rt))
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}

// head returns the first n lines of s, with an ellipsis marker when it truncates, so a long
// emitted document shows its shape without flooding the walkthrough output.
func head(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n") + "\n  ..."
}
