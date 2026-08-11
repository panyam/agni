// Command resolve-design is the walkthrough of the engine's design descriptors: point at a file,
// learn which design it belongs to, and read the design's declared entry rather than whatever file
// happened to be named. The narration lives in the sidecar walkthrough.md (loaded via demokit's
// FromMarkdown), so this file only binds the steps that run engine code and wires the renderer.
//
// Run modes (see the Makefile): `make run` (plain text), `make demo` (TUI boxes),
// `make runquiet` (non-interactive defaults, CI-safe), `make doc` (render to markdown).
package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/panyam/demokit"

	"github.com/panyam/agni/examples/common"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/internal/service"
)

//go:embed walkthrough.md
var walkthroughMD []byte

// fixtureRoot is the bundled project the walkthrough resolves against. A store's tree is rooted at
// one filesystem, which on a server is a mount; here it is this folder. Everything the example names
// is relative to it, never a host path.
const fixtureRoot = "../common/designs/demo-project"

// mount is the name this one tree is addressed by. A ref is always a (mount, ref) pair, so even a
// single-tree client names its tree rather than inventing a path-shaped alternative.
const mount = "fixtures"

func main() {
	// The ref the walkthrough resolves. It defaults to the BOARD rather than the netlist on purpose:
	// the board is the file whose resolution is interesting, because it is a declared companion.
	ref := common.AskPath("ref", "designs/mixer/mixer.kicad_pcb")

	demo := demokit.New("resolve-design").
		Dir("resolve-design").
		FromMarkdownBytes(walkthroughMD)

	// The same ProjectService a server hosts, over the same filesystem-backed store, differing only
	// in which tree it was pointed at. That is the whole shape: clients pick a store, everyone asks
	// the service.
	svc := service.NewProjectService(projects.NewFSStore(projects.Tree{Mount: mount, FS: os.DirFS(fixtureRoot)}))
	ctx := context.Background()

	demo.Bind("pick").Input(ref.Def()).Run(func(c demokit.StepContext) *demokit.StepResult {
		ref.Capture(c)
		fmt.Printf("Resolving %q inside %s.\n", ref.Path(), fixtureRoot)
		return nil
	})

	demo.Bind("list").Run(func(demokit.StepContext) *demokit.StepResult {
		ps, err := svc.ListProjects(ctx, &webapi.ListProjectsRequest{})
		if err != nil {
			return demokit.Errf("list projects: %v", err)
		}
		for _, p := range ps.GetProjects() {
			fmt.Printf("%-34s %s\n", p.GetName(), p.GetTitle())
			ds, err := svc.ListDesigns(ctx, &webapi.ListProjectDesignsRequest{Parent: p.GetName()})
			if err != nil {
				return demokit.Errf("list designs: %v", err)
			}
			for _, d := range ds.GetDesigns() {
				fmt.Printf("  %-32s %s\n", d.GetName(), d.GetTitle())
			}
		}
		return nil
	})

	demo.Bind("resolve").Run(func(demokit.StepContext) *demokit.StepResult {
		resp, err := svc.ResolveDesign(ctx, &webapi.ResolveDesignRequest{Mount: mount, Ref: ref.Path()})
		if err != nil {
			return demokit.Errf("resolve: %v", err)
		}
		d := resp.GetDesign()
		if d == nil {
			// Not an error. Most files in a tree belong to no declared design, and a caller that
			// gets nothing back falls through to the plain behaviour and the built-in catalog.
			fmt.Printf("%s belongs to no declared design.\n", ref.Path())
			return nil
		}
		fmt.Printf("resource:   %s\n", d.GetName())
		fmt.Printf("project:    %s\n", resp.GetProject().GetName())
		fmt.Printf("entry:      %s\n", d.GetEntryRef())
		fmt.Printf("companions: %s\n", strings.Join(d.GetCompanionRefs(), ", "))
		// Which artifact each TIER opens is decided once, above every client, so the CLI and the
		// browser cannot disagree about which companion supplies the board.
		src := service.SourcesFor(d, "")
		fmt.Printf("tiers:      netlist=%s board=%s sheets=%s\n", src.NetlistRef, src.BoardRef, src.GeometryRef)
		return nil
	})

	demo.Bind("read").Run(func(demokit.StepContext) *demokit.StepResult {
		named := ref.Path()
		entry := named
		if resp, err := svc.ResolveDesign(ctx, &webapi.ResolveDesignRequest{Mount: mount, Ref: named}); err == nil && resp.GetDesign() != nil {
			entry = resp.GetDesign().GetEntryRef()
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
