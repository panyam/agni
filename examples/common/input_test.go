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
