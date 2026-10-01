package common

import (
	"os"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/demokit"
)

// PathInput asks the user for a design file path and turns it into an IR design, so every example
// prompts the same way. The path is relative to the example directory, Enter keeps the shown
// default, and whatever is chosen is read with Load.
//
// Wire it into a walkthrough in three touches:
//
//	design := common.AskPath("design", "../common/designs/foo.edn")
//	demo.Bind("pick").Input(design.Def()).Run(func(ctx demokit.StepContext) *demokit.StepResult {
//	    design.Capture(ctx)          // record what the user typed
//	    return nil
//	})
//	demo.Bind("run").Run(func(ctx demokit.StepContext) *demokit.StepResult {
//	    d, err := design.Load()      // read the chosen design in a later step
//	    ...
//	})
type PathInput struct {
	key  string
	path string
}

// DesignPathEnv overrides the default path every AskPath offers, so a walkthrough can run over a
// design this repo cannot carry without that design's path appearing in any file here.
//
// It changes the DEFAULT, not the value, so the prompt still shows the path and the user can still
// type another. --non-interactive runs on it too.
const DesignPathEnv = "AGNI_EXAMPLE_DESIGN"

// ReviewPathEnv overrides the default review manifest, the way DesignPathEnv overrides the design.
// A review walkthrough over real work needs BOTH, and they are separate because one manifest is often
// shared across the designs of a project.
const ReviewPathEnv = "AGNI_EXAMPLE_REVIEW"

// AskPath creates a PathInput bound to the named walkthrough input, defaulting to def (a path
// relative to the example directory, e.g. "../common/designs/foo.edn"), or to DesignPathEnv when
// that is set to a non-blank value.
func AskPath(name, def string) *PathInput {
	return askPathEnv(name, def, DesignPathEnv)
}

// AskReviewPath is AskPath for a review manifest, reading ReviewPathEnv instead, with the same
// semantics, so a blank value is ignored and a typed path still wins.
func AskReviewPath(name, def string) *PathInput {
	return askPathEnv(name, def, ReviewPathEnv)
}

func askPathEnv(name, def, env string) *PathInput {
	if v := strings.TrimSpace(os.Getenv(env)); v != "" {
		def = v
	}
	return &PathInput{key: name, path: def}
}

// Def is the demokit input to attach to the step that collects the path. Declaring it here rather
// than in a markdown `inputs` block keeps the prompt wording identical across examples.
func (p *PathInput) Def() demokit.InputDef {
	return demokit.String().Named(p.key, "Path to a design (relative to this folder)").WithDefault(p.path)
}

// Capture records the path the user entered; call it in the collecting step's Run. A blank
// entry keeps the default.
func (p *PathInput) Capture(ctx demokit.StepContext) {
	if v, ok := ctx.Inputs[p.key].(string); ok && strings.TrimSpace(v) != "" {
		p.path = v
	}
}

// Path is the currently selected path (the default until Capture records an entry).
func (p *PathInput) Path() string { return p.path }

// Load reads the selected design via Load (disk path first, bundled fixture fallback).
func (p *PathInput) Load() (*ir.Design, error) { return Load(p.path) }
