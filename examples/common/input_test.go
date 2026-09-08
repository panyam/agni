package common

import (
	"testing"

	"github.com/panyam/demokit"
)

func TestPathInputCapture(t *testing.T) {
	p := AskPath("design", "designs/two-resistors.edn")
	if p.Path() != "designs/two-resistors.edn" {
		t.Fatalf("default Path() = %q", p.Path())
	}
	// A blank entry keeps the default.
	p.Capture(demokit.StepContext{Inputs: map[string]any{"design": "   "}})
	if p.Path() != "designs/two-resistors.edn" {
		t.Errorf("blank entry changed Path() to %q, want the default", p.Path())
	}
	// A non-blank entry overrides.
	p.Capture(demokit.StepContext{Inputs: map[string]any{"design": "designs/i2c-sensor.edn"}})
	if p.Path() != "designs/i2c-sensor.edn" {
		t.Errorf("Path() = %q, want the entered path", p.Path())
	}
}

func TestPathInputDef(t *testing.T) {
	def := AskPath("design", "x.edn").Def()
	if def.Name != "design" {
		t.Errorf("Def().Name = %q, want design", def.Name)
	}
	if def.Default != "x.edn" {
		t.Errorf("Def().Default = %v, want x.edn", def.Default)
	}
}

func TestPathInputLoad(t *testing.T) {
	d, err := AskPath("design", "designs/two-resistors.edn").Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if d.Name != "DEMO" {
		t.Errorf("Name = %q, want DEMO", d.Name)
	}
}

// TestAskPathEnvDefault: AGNI_EXAMPLE_DESIGN replaces the bundled default, so a walkthrough can be
// driven over a design this repo cannot carry without that path being typed or committed.
func TestAskPathEnvDefault(t *testing.T) {
	t.Setenv(DesignPathEnv, "/somewhere/else/board.edn")
	p := AskPath("design", "../common/designs/two-resistors.edn")
	if p.Path() != "/somewhere/else/board.edn" {
		t.Errorf("Path() = %q, want the environment's value", p.Path())
	}
	if got := p.Def().Default; got != "/somewhere/else/board.edn" {
		t.Errorf("Def().Default = %v, want the environment's value so --non-interactive uses it too", got)
	}
}

// TestAskPathEnvBlankKeepsTheBundledDefault: an exported-but-empty variable is not a value. Without
// this an `export AGNI_EXAMPLE_DESIGN=` in a shell would point every example at "".
func TestAskPathEnvBlankKeepsTheBundledDefault(t *testing.T) {
	for _, v := range []string{"", "   "} {
		t.Setenv(DesignPathEnv, v)
		if p := AskPath("design", "designs/two-resistors.edn"); p.Path() != "designs/two-resistors.edn" {
			t.Errorf("with %q set, Path() = %q, want the bundled default", v, p.Path())
		}
	}
}

// TestAskPathEnvIsStillOverridableByTheUser: the variable moves the DEFAULT, not the value, so the
// prompt still shows it and typing a path still wins.
func TestAskPathEnvIsStillOverridableByTheUser(t *testing.T) {
	t.Setenv(DesignPathEnv, "/somewhere/else/board.edn")
	p := AskPath("design", "designs/two-resistors.edn")
	p.Capture(demokit.StepContext{Inputs: map[string]any{"design": "designs/i2c-sensor.edn"}})
	if p.Path() != "designs/i2c-sensor.edn" {
		t.Errorf("Path() = %q, want what the user typed", p.Path())
	}
}
