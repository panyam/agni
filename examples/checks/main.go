// Command checks is the checks rung of the Agni examples ladder, which runs the structural rule
// checks over a design and reads the findings. It is the walkthrough form of `agni check`.
// The narration lives in the sidecar walkthrough.md (demokit FromMarkdown); this file only
// binds the steps that run engine code.
//
// Run modes (see the Makefile): `make run` (plain text), `make demo` (TUI boxes),
// `make runquiet` (non-interactive defaults, CI-safe), `make doc` (render to markdown).
package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"github.com/panyam/demokit"
	"github.com/panyam/agni/core/check"
	_ "github.com/panyam/agni/stdlib/rules/builtin" // register the built-in rule catalog (check.RunDesign runs it)
	"github.com/panyam/agni/stdlib/rules/intent"
	"github.com/panyam/agni/examples/common"
)

//go:embed walkthrough.md
var walkthroughMD []byte

func main() {
	design := common.AskPath("design", "../common/designs/i2c-sensor/i2c-sensor.edn")

	demo := demokit.New("checks").
		Dir("checks").
		FromMarkdownBytes(walkthroughMD)

	demo.Bind("pick").Input(design.Def()).Run(func(ctx demokit.StepContext) *demokit.StepResult {
		design.Capture(ctx)
		fmt.Printf("Selected %s.\n", design.Path())
		return nil
	})

	demo.Bind("run").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := design.Load()
		if err != nil {
			return demokit.Errf("load %s: %v", design.Path(), err)
		}
		fmt.Println(common.FindingsLines(check.RunDesign(d)))
		return nil
	})

	// The pull-up reach section reads its own fixture and declaration whatever design was picked,
	// because the point is one board read twice, once at the default reach and once as declared.
	demo.Bind("reach").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := common.Load("../common/designs/pullup-reach.edn")
		if err != nil {
			return demokit.Errf("load pullup-reach.edn: %v", err)
		}
		b, err := os.ReadFile("../common/designs/pullup-reach.intent.yaml")
		if err != nil {
			return demokit.Errf("read the declaration: %v", err)
		}
		declared, err := intent.ParseProto(b)
		if err != nil {
			return demokit.Errf("load the declaration: %v", err)
		}
		var rule *check.Rule
		for _, r := range check.BuiltinRules() {
			if r.Name == "i2c-pull-up" {
				rule = r
			}
		}
		if rule == nil {
			return demokit.Errf("the built-in catalog has no i2c-pull-up")
		}
		for _, run := range []struct {
			label string
			model check.Model
		}{
			{"At the default reach:", check.NewModel(d)},
			{"With SDA_MAIN declared to reach 4:", check.NewModel(d, check.WithIntent(declared))},
		} {
			fmt.Println(run.label)
			for _, v := range rule.Eval(context.Background(), run.model) {
				// i2c-pull-up gives every verdict a witness, pass or fail.
				fmt.Printf("  %-4s  %-8s  %s\n", v.Outcome, check.EntityRef(v.Subjects[0]), v.Witness.Statement)
				for _, t := range v.Witness.Terms {
					if t.Label == "path" {
						fmt.Printf("                  path %s\n", t.Value)
					}
				}
			}
		}
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}
