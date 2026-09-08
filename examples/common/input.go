package common

import (
	"os"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/demokit"
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

// DesignPathEnv overrides the default path every AskPath offers. It exists so a walkthrough can be
// driven over a design this repo cannot carry, without that design's path appearing in any file here:
// the board is named in the environment, and what is committed still defaults to a bundled fixture.
//
// Named for EXAMPLES rather than for any one walkthrough, because every example reaches it through
// AskPath and a demo-shaped name would read as belonging to whichever one you happened to be running.
//
// It changes the DEFAULT, not the value, so the prompt still shows the path and the user can still
// type another. --non-interactive then runs on it too, which is what makes the env var worth having
// over just typing the path.
const DesignPathEnv = "AGNI_EXAMPLE_DESIGN"

// ReviewPathEnv overrides the default review manifest, the way DesignPathEnv overrides the design.
//
// A review walkthrough needs BOTH to be pointed at real work: a team's checklist is as unshippable
// here as their board, and a checklist without its design answers nothing. Two variables rather than
// one because the two are separately useful, and because a manifest is often shared across the
// designs of one project where the design is not.
const ReviewPathEnv = "AGNI_EXAMPLE_REVIEW"

// AskPath creates a PathInput bound to the named walkthrough input, defaulting to def (a path
// relative to the example directory, e.g. "../common/designs/foo.edn"), or to DesignPathEnv when
// that is set to a non-blank value.
func AskPath(name, def string) *PathInput {
	return askPathEnv(name, def, DesignPathEnv)
}

// AskReviewPath is AskPath for a review manifest, reading ReviewPathEnv instead. Same semantics
// throughout: a blank value is not a value, the variable moves the DEFAULT rather than the answer, so
// the prompt still shows it and a typed path still wins.
func AskReviewPath(name, def string) *PathInput {
	return askPathEnv(name, def, ReviewPathEnv)
}

func askPathEnv(name, def, env string) *PathInput {
	if v := strings.TrimSpace(os.Getenv(env)); v != "" {
		def = v
	}
	return &PathInput{key: name, path: def}
}

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
