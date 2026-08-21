package common

import (
	"slices"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/diff"
	_ "github.com/panyam/agni/stdlib/rules/builtin" // register the built-in rule catalog (check.RunDesign runs it)
)

func TestDesigns(t *testing.T) {
	got := Designs()
	// Contains the fixtures the examples rely on; other examples add more over time, so this
	// is a subset check, not an exact-list assertion.
	for _, name := range []string{
		"demo-board.ipc2581.xml", "demo-board.kicad_pcb", "two-resistors.edn", "i2c-sensor.edn",
	} {
		if !slices.Contains(got, name) {
			t.Errorf("Designs() missing %q; got %v", name, got)
		}
	}
	if !slices.IsSorted(got) {
		t.Errorf("Designs() is not sorted: %v", got)
	}
}

func TestReadFixtureEDIF(t *testing.T) {
	d, err := ReadFixture("two-resistors.edn")
	if err != nil {
		t.Fatalf("ReadFixture: %v", err)
	}
	if d.Name != "DEMO" {
		t.Errorf("Name = %q, want DEMO", d.Name)
	}
	if len(d.Components) != 5 {
		t.Errorf("Components = %d, want 5 (R1,R2,C1,U1,J1)", len(d.Components))
	}
	if len(d.Nets) != 4 {
		t.Errorf("Nets = %d, want 4 (VCC,GND,SIG,OUT)", len(d.Nets))
	}
}

func TestReadFixtureBoardFormats(t *testing.T) {
	for _, name := range []string{"demo-board.kicad_pcb", "demo-board.ipc2581.xml"} {
		d, err := ReadFixture(name)
		if err != nil {
			t.Fatalf("ReadFixture(%s): %v", name, err)
		}
		if len(d.Components) == 0 {
			t.Errorf("ReadFixture(%s): expected components, got 0", name)
		}
	}
}

func TestReadFixtureNotFound(t *testing.T) {
	if _, err := ReadFixture("does-not-exist.edn"); err == nil {
		t.Error("ReadFixture of a missing fixture should error")
	}
}

// TestI2CSensorFindings pins the checks-example fixture: it must trip exactly one finding at
// each severity, and leave the two deliberately-benign nets unflagged (SDA has a pull-up;
// NC_SPARE is a no-connect). Compared as findings, not formatted text.
func TestI2CSensorFindings(t *testing.T) {
	d, err := ReadFixture("i2c-sensor.edn")
	if err != nil {
		t.Fatalf("ReadFixture: %v", err)
	}
	fs := check.RunDesign(d)
	if len(fs) != 3 {
		t.Fatalf("check.Run = %d findings, want 3:\n%+v", len(fs), fs)
	}
	type key struct{ rule, sev, subj string }
	seen := map[key]bool{}
	for _, f := range fs {
		seen[key{f.Rule, f.Severity, check.EntityRef(f.Subject)}] = true
	}
	for _, want := range []key{
		{"i2c-pull-up", "error", "SCL"},
		{"single-pin-net", "info", "STUB"},
		{"unconnected-component", "warning", "C1"},
	} {
		if !seen[want] {
			t.Errorf("missing finding %+v; got %+v", want, fs)
		}
	}
	for _, f := range fs {
		if check.EntityRef(f.Subject) == "SDA" || check.EntityRef(f.Subject) == "NC_SPARE" {
			t.Errorf("%s should be suppressed (SDA has a pull-up; NC_SPARE is a no-connect), got %+v", f.Subject, f)
		}
	}
}

// TestRevPairDiff pins the diff-example fixture pair: rev-a -> rev-b must produce one change of
// each class (renamed, hard, new, deleted) plus one added component, with the unchanged nets
// staying quiet. Compared as Report fields, not the rendered string.
func TestRevPairDiff(t *testing.T) {
	a, err := ReadFixture("rev-a.edn")
	if err != nil {
		t.Fatalf("ReadFixture(rev-a): %v", err)
	}
	b, err := ReadFixture("rev-b.edn")
	if err != nil {
		t.Fatalf("ReadFixture(rev-b): %v", err)
	}
	r := diff.Designs(a, b)

	if !slices.Equal(r.ComponentsAdded, []string{"R4"}) {
		t.Errorf("ComponentsAdded = %v, want [R4]", r.ComponentsAdded)
	}
	if len(r.ComponentsRemoved) != 0 {
		t.Errorf("ComponentsRemoved = %v, want none", r.ComponentsRemoved)
	}

	kind := map[string]diff.NetChangeKind{}
	for _, n := range r.Nets {
		kind[n.Name] = n.Kind
	}
	want := map[string]diff.NetChangeKind{
		"DATA": diff.NetRenamed, // from SIG
		"CLK":  diff.NetHard,
		"NEW":  diff.NetNew,
		"OLD":  diff.NetDeleted,
	}
	for name, w := range want {
		if kind[name] != w {
			t.Errorf("net %s kind = %q, want %q (nets: %+v)", name, kind[name], w, r.Nets)
		}
	}
	// Unchanged nets are not reported.
	for _, n := range r.Nets {
		if n.Name == "VCC" || n.Name == "GND" {
			t.Errorf("equal net %s should not appear in the diff, got %+v", n.Name, n)
		}
	}
	// The rename records the old name.
	for _, n := range r.Nets {
		if n.Name == "DATA" && n.OldName != "SIG" {
			t.Errorf("DATA rename OldName = %q, want SIG", n.OldName)
		}
	}
}

// TestMixerConvergence pins the multi-format example fixtures: the same board read from EDIF,
// KiCad, and IPC-2581 must yield the identical netlist. diff.Designs over the pairs is the
// oracle: zero net changes and the same component set. Component attributes and the physical
// tier are allowed to differ (each format carries different metadata).
func TestMixerConvergence(t *testing.T) {
	edn, err := ReadFixture("mixer.edn")
	if err != nil {
		t.Fatalf("ReadFixture(mixer.edn): %v", err)
	}
	if len(edn.Components) != 3 || len(edn.Nets) != 3 {
		t.Fatalf("mixer.edn = %d components / %d nets, want 3/3", len(edn.Components), len(edn.Nets))
	}
	for _, name := range []string{"mixer.kicad_pcb", "mixer.ipc2581.xml"} {
		d, err := ReadFixture(name)
		if err != nil {
			t.Fatalf("ReadFixture(%s): %v", name, err)
		}
		if len(d.Components) != 3 || len(d.Nets) != 3 {
			t.Errorf("%s = %d components / %d nets, want 3/3", name, len(d.Components), len(d.Nets))
		}
		r := diff.Designs(edn, d)
		if len(r.Nets) != 0 {
			t.Errorf("EDIF vs %s: %d net change(s), want 0 (netlists should converge): %+v", name, len(r.Nets), r.Nets)
		}
		if len(r.ComponentsAdded) != 0 || len(r.ComponentsRemoved) != 0 {
			t.Errorf("EDIF vs %s: components +%v -%v, want the same set", name, r.ComponentsAdded, r.ComponentsRemoved)
		}
	}
}
