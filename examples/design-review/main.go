// Command design-review is the review rung of the Agni examples ladder: a team's schematic-review
// checklist, run against a board, with every item resolving to an outcome including the ones nothing
// can answer.
//
// It is the walk that COMPOSES the others. A checklist item binds to a rule (rung 3), to a query over
// the fact relations (rung 11), or to nothing at all, so this is where those surfaces stop being
// separate features and become answers to questions someone wrote down. The last step is the reason
// it exists: a report that only shows what it could answer is indistinguishable from one that asked
// less.
//
// The narration lives in the sidecar walkthrough.md (demokit FromMarkdown); this file only binds the
// steps that run engine code.
//
// Run modes (see the Makefile): `make run` (plain text), `make demo` (TUI boxes),
// `make runquiet` (non-interactive defaults, CI-safe), `make doc` (render to markdown).
package main

import (
	_ "embed"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/review"
	"github.com/panyam/agni/examples/common"
	_ "github.com/panyam/agni/stdlib/relations"     // register the fact relations a query item binds
	_ "github.com/panyam/agni/stdlib/reviewquery"   // register the datalog query compiler; an inline query item fails to COMPILE without it
	_ "github.com/panyam/agni/stdlib/rules/builtin" // register the rule catalog; BuiltinRules is EMPTY without it, silently
	"github.com/panyam/demokit"
)

//go:embed walkthrough.md
var walkthroughMD []byte

// checklistYAML is bundled rather than read from disk, so the walk runs from any directory with no
// arguments. AGNI_EXAMPLE_REVIEW replaces it with a path, the way AGNI_EXAMPLE_DESIGN replaces the
// design: a team's checklist is as unshippable here as their board.
//
//go:embed checklist.yaml
var checklistYAML []byte

func main() {
	design := common.AskPath("design", "../common/designs/i2c-sensor/i2c-sensor.edn")
	checklist := common.AskReviewPath("checklist", "checklist.yaml")

	demo := demokit.New("design-review").
		Dir("design-review").
		FromMarkdownBytes(walkthroughMD)

	demo.Bind("pick").Input(design.Def()).Input(checklist.Def()).Run(func(ctx demokit.StepContext) *demokit.StepResult {
		design.Capture(ctx)
		checklist.Capture(ctx)
		fmt.Printf("Design    %s\nChecklist %s\n", design.Path(), checklist.Path())
		return nil
	})

	demo.Bind("coverage").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		rep, err := run(design, checklist)
		if err != nil {
			return demokit.Errf("%v", err)
		}
		cli("agni review <design> --checklist <checklist> --coverage")
		byOutcome := tally(rep)
		covered, answered := 0, 0
		for _, it := range items(rep) {
			if it.Outcome != review.NotAutomated {
				covered++
			}
			if it.Outcome == review.Pass || it.Outcome == review.Fail {
				answered++
			}
		}
		fmt.Printf("%d of %d covered, %d answered.\n\n", covered, len(items(rep)), answered)
		for _, k := range sortedKeys(byOutcome) {
			fmt.Printf("  %2d  %s\n", byOutcome[k], k)
		}
		return nil
	})

	demo.Bind("drill").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		rep, err := run(design, checklist)
		if err != nil {
			return demokit.Errf("%v", err)
		}
		cli("agni check <design> --verdicts")
		for _, it := range items(rep) {
			if it.Outcome != review.Fail {
				continue
			}
			fmt.Printf("%s %s\n  rests on: %s\n", it.Item.ID, it.Item.Title, binding(it))
			for _, f := range it.Findings {
				fmt.Printf("    %s\n", f.Message)
			}
			return nil
		}
		fmt.Println("Nothing failed on this design.")
		return nil
	})

	demo.Bind("house").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		rep, err := run(design, checklist)
		if err != nil {
			return demokit.Errf("%v", err)
		}
		for _, it := range items(rep) {
			if it.Item.Query == nil {
				continue
			}
			cli("agni query <design> '" + it.Item.Query.Match + "'")
			fmt.Printf("%s %s -> %s\n", it.Item.ID, it.Item.Title, it.Outcome)
			fmt.Println("\nThe item carries its own question, so the convention is data rather than code.")
			return nil
		}
		return nil
	})

	demo.Bind("gap").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		rep, err := run(design, checklist)
		if err != nil {
			return demokit.Errf("%v", err)
		}
		// Three reasons an item is unanswered, and only one is a gap in the tool. Printing them
		// together is the point of the step: a coverage number that folded them into "not passing"
		// would say the same thing about a missing rule and a missing declaration.
		for _, it := range items(rep) {
			switch it.Outcome {
			case review.NotApplicable:
				fmt.Printf("  %-3s %-40s the rule ran and this design has no such tier\n", it.Item.ID, it.Item.Title)
			case review.NeedsDesignIntent:
				fmt.Printf("  %-3s %-40s the rule exists; nobody declared what to check against\n", it.Item.ID, it.Item.Title)
			case review.NotAutomated:
				fmt.Printf("  %-3s %-40s NOTHING covers this. It stays on the list.\n", it.Item.ID, it.Item.Title)
			}
		}
		fmt.Println("\nOnly the last is a gap in the tool. Removing it would raise the covered count")
		fmt.Println("and lower what that count is worth.")
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}

// run reads the design and the checklist and evaluates one against the other. Re-run per step rather
// than cached, matching the other walkthroughs: a step reads what it needs, so a reader can run one
// in isolation and the narration never depends on an earlier step having happened.
func run(design, checklist *common.PathInput) (review.Report, error) {
	d, err := design.Load()
	if err != nil {
		return review.Report{}, fmt.Errorf("load %s: %w", design.Path(), err)
	}
	man, err := review.Load(strings.NewReader(manifestText(checklist)))
	if err != nil {
		return review.Report{}, fmt.Errorf("checklist %s: %w", checklist.Path(), err)
	}
	// NewModelWithParams, not NewModel: component.mpn reads the map the params constructor fills, so
	// the house-rule query below would find no part numbers on a design whose parts all carry one. A
	// nil provider is fine; only the datasheet relations need a real one.
	// The BUILT-IN catalog, with no project overlay. A project's own interface profiles, design
	// intent and naming conventions each answer items this cannot, so the covered count here is lower
	// than  reports on the same two files (123 of 302 against 76, on one real board).
	// Composing an overlay is the CLI's job and takes a project descriptor; the walkthrough is about
	// what a checklist REPORTS, and it reports the same shape either way.
	return review.Run(review.RunParams{
		Model:    check.NewModelWithParams(d, nil, nil),
		Catalog:  check.DefaultCatalog(),
		Manifest: man,
		Design:   design.Path(),
	}), nil
}

// manifestText prefers the file the reader named and falls back to the bundled copy, the same
// disk-first rule PathInput.Load applies to a design.
func manifestText(checklist *common.PathInput) string {
	if b, err := os.ReadFile(checklist.Path()); err == nil {
		return string(b)
	}
	return string(checklistYAML)
}

// items flattens the report's areas, since every step here reads across them.
func items(rep review.Report) []review.ItemResult {
	var out []review.ItemResult
	for _, a := range rep.Areas {
		out = append(out, a.Items...)
	}
	return out
}

// binding names what an item resolved through, for the drill step.
func binding(it review.ItemResult) string {
	switch {
	case it.Item.Rule != "":
		return "rule " + it.Item.Rule
	case it.Item.Query != nil:
		return "an inline query"
	case it.Item.Present != nil:
		return "a class-presence check"
	}
	return "nothing"
}

func tally(rep review.Report) map[string]int {
	out := map[string]int{}
	for _, it := range items(rep) {
		out[string(it.Outcome)]++
	}
	return out
}

func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// cli prints the command a reader can paste into a second terminal, so nothing the walkthrough shows
// is reachable only from Go.
func cli(line string) { fmt.Printf("$ %s\n\n", line) }
