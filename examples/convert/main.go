// Command convert is the emit rung of the Agni examples ladder. It reads a supported source
// format into the neutral IR and emits it back out, so a conversion is A -> IR -> B. The only
// writer it offers is IPC-2581, and it re-reads the output to check the round trip.
//
// The narration lives in the sidecar walkthrough.md (demokit FromMarkdown); this file only binds
// the steps that run engine code.
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
	"github.com/panyam/agni/readers/ipc2581"
)

//go:embed walkthrough.md
var walkthroughMD []byte

func main() {
	// chosen and format carry the picks to the emit step. Their defaults must match the markdown
	// inputs' defaults, or a non-interactive run reads one design and reports another.
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

		// Matching stats on the re-read show the modeled IR survived.
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

// head returns the first n lines of s, with an ellipsis marker when it truncates.
func head(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n") + "\n  ..."
}
