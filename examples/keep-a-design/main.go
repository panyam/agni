// Command keep-a-design is the walkthrough of the designs a server keeps (agni issues 1694, 1696).
// It stores two revisions of a board as designs of one series, each holding a netlist artifact, keeps a
// pack of house rules and a report under it (agni issue 1704), groups the designs in a project with a design
// from a mounted folder, and shows what a mounted folder will not let it change. Last, it links a
// design to a folder and lets a poll follow the folder's changes (agni issue 1726), and does the same
// for a pack read from a project folder (agni issue 1801). The narration is in
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
	"path/filepath"

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

	demo.Bind("pack").Run(func(demokit.StepContext) *demokit.StepResult {
		before, after, err := t.keepPack(ctx)
		if err != nil {
			return demokit.Errf("keep the pack: %v", err)
		}
		fmt.Printf("kept %s, %q\n  hash %s\n", before.GetName(), before.GetTitle(), before.GetHash())
		fmt.Printf("after its rail pattern changed\n  hash %s\n", after.GetHash())
		return nil
	})

	demo.Bind("report").Run(func(demokit.StepContext) *demokit.StepResult {
		kept, after, err := t.report(ctx)
		if err != nil {
			return demokit.Errf("report: %v", err)
		}
		fmt.Printf("kept %s\n  packs %v, %d findings\n", kept.GetName(), kept.GetPacks(), len(kept.GetResults().GetFindings()))
		fmt.Printf("after rev B's netlist was replaced\n  design_changed %v\n", after.GetDesignChanged())
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

	demo.Bind("follow").Run(func(demokit.StepContext) *demokit.StepResult {
		linked, synced, marked, err := t.follow(ctx)
		if err != nil {
			return demokit.Errf("follow the lab folder: %v", err)
		}
		fmt.Printf("linked to the folder holding rev A\n  version %s\n", linked.GetVersion())
		fmt.Printf("after rev B replaced the folder's netlist, a poll synced it\n  version %s\n", synced.GetVersion())
		fmt.Printf("after rev A came back, with the design set to mark\n  version %s\n  newer_version %s\n", marked.GetVersion(), marked.GetNewerVersion())
		return nil
	})

	demo.Bind("followpack").Run(func(demokit.StepContext) *demokit.StepResult {
		linked, moved, err := t.followPack(ctx)
		if err != nil {
			return demokit.Errf("follow the rules folder: %v", err)
		}
		fmt.Printf("linked packs/house-live to the rules folder\n  %d house rule, hash %s\n", len(linked.GetConfig().GetConventions().GetRules()), linked.GetHash())
		fmt.Printf("after a rule was added to its project.yaml, a poll moved it\n  %d house rules, hash %s\n", len(moved.GetConfig().GetConventions().GetRules()), moved.GetHash())
		return nil
	})

	demo.Bind("sqlite").Run(func(demokit.StepContext) *demokit.StepResult {
		lite, recs, closeDB, err := sqliteTour(filepath.Join(root, "sqlite"), mounted)
		if err != nil {
			return demokit.Errf("open the sqlite store: %v", err)
		}
		defer closeDB()
		for _, rev := range []struct{ id, title, netlist string }{
			{"mixer-rev-a", "Mixer rev A", "../common/designs/rev-a.edn"},
			{"mixer-rev-b", "Mixer rev B", "../common/designs/rev-b.edn"},
		} {
			if _, _, err := lite.keep(ctx, rev.id, rev.title, rev.netlist); err != nil {
				return demokit.Errf("keep %s in sqlite: %v", rev.id, err)
			}
		}
		ks, err := keys(ctx, recs, "")
		if err != nil {
			return demokit.Errf("list the records: %v", err)
		}
		fmt.Printf("agni.db holds %d records:\n", len(ks))
		for _, k := range ks {
			fmt.Println("  " + k)
		}
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}
