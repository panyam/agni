package composetest

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/panyam/agni"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/readers/formats"
)

// RunAll evaluates each rule once (agni issue 810) and must return exactly what Run and RunVerdicts
// return over the same rules, separately. Checked over the whole composed catalog on the tutorial's
// small board and on the real 1123-component sample board, whose catalog run is what the change
// exists to make cheaper. Each side gets its own model, so a value one side cached cannot help the
// other agree.
func TestRunAllMatchesRunAndRunVerdicts(t *testing.T) {
	e, err := agni.New()
	if err != nil {
		t.Fatal(err)
	}
	rules := e.Catalog().Rules()
	for name, path := range map[string]string{
		"gateway": "../../examples/tutorial-project/designs/gateway/gateway.edn",
		"jetson":  "../../tools/samples/boards/jetson-agx-thor-baseboard/jetson-agx-thor-baseboard.kicad_sch",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := os.Stat(path); err != nil {
				// Fatal rather than skipped: a missing corpus must not read as a pass.
				t.Fatalf("%s missing (run `make samples` for the sample board): %v", path, err)
			}
			d, err := (&formats.Loader{}).ReadDesign(path)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			wantF, err := check.Run(ctx, check.NewModel(d), rules)
			if err != nil {
				t.Fatal(err)
			}
			wantV, err := check.RunVerdicts(ctx, check.NewModel(d), rules)
			if err != nil {
				t.Fatal(err)
			}
			gotF, gotV, err := check.RunAll(ctx, check.NewModel(d), rules)
			if err != nil {
				t.Fatal(err)
			}
			if len(wantF) == 0 || len(wantV) == 0 {
				t.Fatalf("the catalog produced %d findings and %d verdicts; an empty run proves nothing", len(wantF), len(wantV))
			}
			if !reflect.DeepEqual(gotF, wantF) {
				t.Errorf("RunAll findings differ from Run's: %d against %d", len(gotF), len(wantF))
			}
			if !reflect.DeepEqual(gotV, wantV) {
				t.Errorf("RunAll verdicts differ from RunVerdicts': %d against %d", len(gotV), len(wantV))
			}
		})
	}
}
