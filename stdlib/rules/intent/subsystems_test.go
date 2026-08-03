package intent

import (
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func TestSubsystemPassesWhenComplete(t *testing.T) {
	decl := declOf(t, `
name: I
subsystems:
  - {name: main clock, source: {class: crystal}, nets: [XTAL_IN, XTAL_OUT]}
`)
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "X1", DeviceClasses: []string{"crystal"}}},
		Nets:       []*ir.Net{{Name: "XTAL_IN"}, {Name: "XTAL_OUT"}},
	}
	if fs := check.Run(check.NewModel(d), Compile(decl)); len(fs) != 0 {
		t.Errorf("a complete subsystem must not fire, got %+v", fs)
	}
}

func TestSubsystemFiresOnMissingSourceAndNet(t *testing.T) {
	decl := declOf(t, `
name: I
subsystems:
  - {name: reset, source: {class: supervisor}, nets: [PORZ, SYS_RESET_N]}
`)
	// The design has PORZ but no supervisor and no SYS_RESET_N: two findings, one per missing piece.
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "U1", DeviceClasses: []string{"ic"}}},
		Nets:       []*ir.Net{{Name: "PORZ"}},
	}
	fs := check.Run(check.NewModel(d), Compile(decl))
	if len(fs) != 2 {
		t.Fatalf("want 2 findings (absent source + absent net), got %d: %+v", len(fs), fs)
	}
	// The rule name is slugified per-subsystem so items bind distinctly.
	for _, f := range fs {
		if f.Rule != "subsystem-reset" {
			t.Errorf("finding should carry the per-subsystem rule name, got %q", f.Rule)
		}
	}
}

func TestSubsystemsCompileToDistinctRules(t *testing.T) {
	decl := declOf(t, `
name: I
subsystems:
  - {name: main clock, source: {class: crystal}}
  - {name: reset, nets: [SYS_RESET_N]}
`)
	rules := Compile(decl)
	names := map[string]bool{}
	for _, r := range rules {
		names[r.Name] = true
	}
	if !names["subsystem-main-clock"] || !names["subsystem-reset"] {
		t.Errorf("each subsystem should compile to its own rule, got %v", names)
	}
}

func TestSubsystemNetsOnlyOrSourceOnly(t *testing.T) {
	// nets-only subsystem (the power-tree shape): every rail must exist.
	decl := declOf(t, "name: I\nsubsystems:\n  - {name: power tree, nets: [5V0, 3V3, 1V8]}")
	d := &ir.Design{Nets: []*ir.Net{{Name: "5V0"}, {Name: "3V3"}}} // 1V8 missing
	fs := check.Run(check.NewModel(d), Compile(decl))
	if len(fs) != 1 || fs[0].Subject != "1V8" || fs[0].Rule != "subsystem-power-tree" {
		t.Fatalf("nets-only subsystem should fire once for the missing rail, got %+v", fs)
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{"main clock": "main-clock", "Reset / Boot": "reset-boot", "power_tree": "power-tree", "A2B": "a2b"}
	for in, want := range cases {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseRejectsSubsystems(t *testing.T) {
	cases := map[string]string{
		"empty subsystem": "name: N\nsubsystems:\n  - {name: x}",
		"no name":         "name: N\nsubsystems:\n  - {nets: [A]}",
		"source no crit":  "name: N\nsubsystems:\n  - {name: x, source: {}}",
		"slug collision":  "name: N\nsubsystems:\n  - {name: main clock, nets: [A]}\n  - {name: 'main  clock', nets: [B]}",
		"non-alnum name":  "name: N\nsubsystems:\n  - {name: '///', nets: [A]}",
	}
	for label, doc := range cases {
		if _, err := Parse([]byte(doc)); err == nil {
			t.Errorf("%s: expected a validation error, got nil", label)
		}
	}
}
