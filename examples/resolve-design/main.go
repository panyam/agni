// Command resolve-design is the walkthrough of the engine's design descriptors. It points at a
// file, learns which design it belongs to, and reads the design's declared entry rather than
// whatever file happened to be named. The narration is in the sidecar walkthrough.md (loaded via
// demokit's FromMarkdown), so this file only binds the steps that run engine code and wires the
// renderer.
//
// The Makefile has four run modes. `make run` prints plain text, `make demo` draws TUI boxes,
// `make runquiet` takes non-interactive defaults (CI-safe), and `make doc` renders to markdown.
package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/panyam/demokit"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/examples/common"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/service"
)

//go:embed walkthrough.md
var walkthroughMD []byte

// fixtureRoot is the bundled project for the walkthrough to resolve against. A store's tree is
// rooted at one filesystem. On a server that is a mount, and here it is this folder. Everything the
// example names is relative to it, never a host path.
const fixtureRoot = "../common/designs/demo-project"

// mount names this one tree. An artifact URI always has an authority, so even a single-tree client
// names its tree rather than inventing a path-shaped alternative.
const mount = "fixtures"

func main() {
	// The ref the walkthrough resolves. It defaults to the BOARD rather than the netlist, because
	// the board is a declared companion and so resolves to a different file than the one named.
	ref := common.AskPath("ref", "designs/mixer/mixer.kicad_pcb")

	demo := demokit.New("resolve-design").
		Dir("resolve-design").
		FromMarkdownBytes(walkthroughMD)

	// A server hosts this same ProjectService over the same filesystem-backed store. Only the tree
	// differs. Clients pick a store, and everyone asks the service.
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
		resp, err := svc.ResolveDesign(ctx, &webapi.ResolveDesignRequest{Uri: artifactUri(ref.Path())})
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
		fmt.Printf("entry:      %s\n", d.GetEntryUri())
		fmt.Printf("companions: %s\n", strings.Join(d.GetCompanionUris(), ", "))
		// SourcesFor decides once, above every client, which artifact each TIER opens. So the CLI
		// and the browser cannot disagree about which companion supplies the board.
		src := service.SourcesFor(d, "")
		fmt.Printf("tiers:      netlist=%s board=%s sheets=%s\n", src.NetlistURI, src.BoardURI, src.GeometryURI)
		return nil
	})

	demo.Bind("read").Run(func(demokit.StepContext) *demokit.StepResult {
		named := artifactUri(ref.Path())
		entry := named
		if resp, err := svc.ResolveDesign(ctx, &webapi.ResolveDesignRequest{Uri: named}); err == nil && resp.GetDesign() != nil {
			entry = resp.GetDesign().GetEntryUri()
		}
		// Read BOTH. On this small fixture the two answers happen to match, and neither says which
		// file it came from, so a divergence on a real export would be invisible. Both sides are
		// addressed the same way, so the comparison is of what was READ rather than how it was
		// spelled. Turning a URI back into a path happens once, here, at the file edge.
		for _, r := range []struct{ label, uri string }{{"as named", named}, {"the design's entry", entry}} {
			u, err := artifact.Parse(r.uri)
			if err != nil {
				return demokit.Errf("parse %s: %v", r.uri, err)
			}
			d, err := common.Load(fixtureRoot + "/" + u.Path)
			if err != nil {
				return demokit.Errf("load %s: %v", r.uri, err)
			}
			fmt.Printf("%-20s %-46s %d components, %d nets\n", r.label, r.uri, len(d.Components), len(d.Nets))
		}
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}

// artifactUri names a file inside the bundled fixture tree. The example declares one mount, so the
// authority is fixed and only the path varies. A server does the same with the operator's mounts.
func artifactUri(rel string) string {
	u, err := artifact.New(mount, rel)
	if err != nil {
		return rel
	}
	return u.String()
}
