// Command resolve-design is the walkthrough of the engine's design descriptors: point at a file,
// learn which design it belongs to, and read the design's declared entry rather than whatever file
// happened to be named. The narration lives in the sidecar walkthrough.md (loaded via demokit's
// FromMarkdown), so this file only binds the steps that run engine code and wires the renderer.
//
// Run modes (see the Makefile): `make run` (plain text), `make demo` (TUI boxes),
// `make runquiet` (non-interactive defaults, CI-safe), `make doc` (render to markdown).
package main

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/panyam/demokit"

	"github.com/panyam/agni/examples/common"
	"github.com/panyam/agni/project"
)

//go:embed walkthrough.md
var walkthroughMD []byte

// fixtureRoot is the bundled project the walkthrough resolves against. A project.Tree is rooted at
// one filesystem, which on a server is a mount; here it is this folder. Everything the example names
// is relative to it, never a host path.
const fixtureRoot = "../common/designs/demo-project"

func main() {
	// The ref the walkthrough resolves. It defaults to the BOARD rather than the netlist on purpose:
	// the board is the file whose resolution is interesting, because it is a declared companion.
	ref := common.AskPath("ref", "designs/mixer/mixer.kicad_pcb")

	demo := demokit.New("resolve-design").
		Dir("resolve-design").
		FromMarkdownBytes(walkthroughMD)

	tree := project.Tree{FS: os.DirFS(fixtureRoot)}

	demo.Bind("pick").Input(ref.Def()).Run(func(c demokit.StepContext) *demokit.StepResult {
		ref.Capture(c)
		fmt.Printf("Resolving %q inside %s.\n", ref.Path(), fixtureRoot)
		return nil
	})

	demo.Bind("list").Run(func(demokit.StepContext) *demokit.StepResult {
		ps, err := tree.Projects()
		if err != nil {
			return demokit.Errf("list projects: %v", err)
		}
		for _, p := range ps {
			fmt.Printf("projects/%-16s %s\n", p.Name, p.DisplayName())
			ds, err := tree.Designs(p.Dir)
			if err != nil {
				return demokit.Errf("list designs: %v", err)
			}
			for _, d := range ds {
				fmt.Printf("  designs/%-14s %s\n", d.Name, d.DisplayName())
			}
		}
		return nil
	})

	demo.Bind("resolve").Run(func(demokit.StepContext) *demokit.StepResult {
		d, p, ok, err := tree.Resolve(ref.Path())
		if err != nil {
			return demokit.Errf("resolve: %v", err)
		}
		if !ok {
			// Not an error. Most files in a tree belong to no declared design, and a caller that
			// gets nothing back falls through to the plain behaviour and the built-in catalog.
			fmt.Printf("%s belongs to no declared design.\n", ref.Path())
			return nil
		}
		fmt.Printf("resource:   projects/%s/designs/%s\n", p.Name, d.Name)
		fmt.Printf("entry:      %s\n", d.EntryRef())
		fmt.Printf("companions: %s\n", strings.Join(d.CompanionRefs(), ", "))
		return nil
	})

	demo.Bind("read").Run(func(demokit.StepContext) *demokit.StepResult {
		named := ref.Path()
		entry := named
		if d, _, ok, err := tree.Resolve(named); err == nil && ok {
			entry = d.EntryRef()
		}
		// Reading BOTH is what makes the point visible. On this small fixture the two answers happen
		// to match; neither of them says which file it came from, which is exactly why a divergence
		// on a real export would be invisible.
		for _, r := range []struct{ label, rel string }{{"as named", named}, {"the design's entry", entry}} {
			d, err := common.Load(fixtureRoot + "/" + r.rel)
			if err != nil {
				return demokit.Errf("load %s: %v", r.rel, err)
			}
			fmt.Printf("%-20s %-34s %d components, %d nets\n", r.label, r.rel, len(d.Components), len(d.Nets))
		}
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}
