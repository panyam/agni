// Command keep-a-design is the walkthrough of the designs a server keeps (agni issues 1694, 1696).
// It stores two revisions of a board as designs of one series, each holding a netlist artifact, groups them in a project with a design
// from a mounted folder, and shows what a mounted folder will not let it change. The narration is in
// the sidecar walkthrough.md, so this file only binds the steps that run engine code.
//
// The Makefile has four run modes. `make run` prints plain text, `make demo` draws TUI boxes,
// `make runquiet` takes non-interactive defaults (CI-safe), and `make doc` renders to markdown.
package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"github.com/panyam/demokit"

	"github.com/panyam/agni/examples/common"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

//go:embed walkthrough.md
var walkthroughMD []byte

// mounted is the bundled demo project, mounted read-only as "fixtures". Its one design is mixer.
const mounted = "../common/designs/demo-project"

func main() {
	root, err := os.MkdirTemp("", "agni-keep-a-design-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(root)
	t, err := newTour(root, mounted)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx := context.Background()
	demo := demokit.New("keep-a-design").Dir("keep-a-design").FromMarkdownBytes(walkthroughMD)

	demo.Bind("keep").Run(func(demokit.StepContext) *demokit.StepResult {
		d, asked, err := t.keep(ctx, "mixer-rev-a", "Mixer rev A", "../common/designs/rev-a.edn")
		if err != nil {
			return demokit.Errf("keep rev A: %v", err)
		}
		fmt.Printf("The server asked for %d file contents, then kept %s.\n", asked, d.GetName())
		fmt.Printf("artifacts: %v\nentry:     %s\nseries:    %s\n", d.GetArtifacts(), d.GetEntryUri(), d.GetSeries())
		return nil
	})

	demo.Bind("again").Run(func(demokit.StepContext) *demokit.StepResult {
		d, _, err := t.keep(ctx, "mixer-rev-b", "Mixer rev B", "../common/designs/rev-b.edn")
		if err != nil {
			return demokit.Errf("keep rev B: %v", err)
		}
		fmt.Printf("kept %s\n", d.GetName())
		list, err := t.designs.ListDesigns(ctx, &webapi.ListDesignsRequest{Filter: `series = "mixer"`})
		if err != nil {
			return demokit.Errf("list the series: %v", err)
		}
		for _, d := range list.GetDesigns() {
			mark := ""
			if d.GetSeriesLatest() {
				mark = "  (latest)"
			}
			fmt.Printf("  %-22s %s%s\n", d.GetName(), d.GetTitle(), mark)
		}
		return nil
	})

	demo.Bind("group").Run(func(demokit.StepContext) *demokit.StepResult {
		names := []string{"designs/mixer", "designs/mixer-rev-a", "designs/mixer-rev-b"}
		if _, err := t.projects.CreateProject(ctx, &webapi.CreateProjectRequest{ProjectId: "lab", Project: &webapi.Project{Title: "Mixer lab", Designs: names}}); err != nil {
			return demokit.Errf("create the project: %v", err)
		}
		list, err := t.designs.ListDesigns(ctx, &webapi.ListDesignsRequest{Filter: `project = "projects/lab"`})
		if err != nil {
			return demokit.Errf("list the project: %v", err)
		}
		for _, d := range list.GetDesigns() {
			fmt.Printf("  %-22s %s\n", d.GetName(), d.GetEntryUri())
		}
		return nil
	})

	demo.Bind("mounted").Run(func(demokit.StepContext) *demokit.StepResult {
		err := t.designs.DeleteDesign(ctx, &webapi.DeleteDesignRequest{Name: "designs/mixer"})
		fmt.Printf("DeleteDesign(designs/mixer): %v\n", err)
		return nil
	})

	demo.Bind("ungroup").Run(func(demokit.StepContext) *demokit.StepResult {
		if err := t.projects.DeleteProject(ctx, &webapi.DeleteProjectRequest{Name: "projects/lab"}); err != nil {
			return demokit.Errf("delete the project: %v", err)
		}
		list, err := t.designs.ListDesigns(ctx, &webapi.ListDesignsRequest{})
		if err != nil {
			return demokit.Errf("list designs: %v", err)
		}
		fmt.Printf("With projects/lab gone, the server still holds %d designs.\n", len(list.GetDesigns()))
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}
