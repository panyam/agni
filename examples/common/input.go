package common

import (
	"strings"

	"github.com/panyam/demokit"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// PathInput is the shared way an example asks the user for a design file path and turns it
// into an IR design, so every example prompts the same way. The path is relative to the
// example directory (the default is shown when the user just presses Enter), and any path the
// user types is loaded with Load (disk first, bundled fixture by base name as a fallback).
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

// AskPath creates a PathInput bound to the named walkthrough input, defaulting to def (a path
// relative to the example directory, e.g. "../common/designs/foo.edn").
func AskPath(name, def string) *PathInput { return &PathInput{key: name, path: def} }

// Def is the demokit input to attach to the step that collects the path. Declaring it here
// (rather than in a markdown `inputs` block) keeps the prompt wording identical across
// examples; the sidecar step carries only its note.
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
