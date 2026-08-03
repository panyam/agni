package common

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/panyam/agni/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// hasRow reports whether s has a "label: N" row (any inner spacing), so the assertions do not
// depend on the exact column alignment.
func hasRow(s, label string, n int) bool {
	return regexp.MustCompile(regexp.QuoteMeta(label) + `:\s+` + fmt.Sprint(n) + `\b`).MatchString(s)
}

func TestStatsLines(t *testing.T) {
	d := &ir.Design{
		Name:         "D",
		SourceFormat: "edif-2.0.0",
		Libraries:    []*ir.PartLibrary{{Name: "LIB"}},
		Components: []*ir.Component{
			{RefDes: "R1", Sections: []*ir.ComponentSection{{}}},
			{RefDes: "U1", Sections: []*ir.ComponentSection{{}, {}}}, // multi-section
		},
		Nets: []*ir.Net{{Name: "GND"}, {Name: "SIG"}, {Name: "VCC"}},
	}
	got := StatsLines(d)
	if !strings.Contains(got, "netlist:") || !strings.Contains(got, "format detail:") {
		t.Errorf("StatsLines missing the netlist / format detail split:\n%s", got)
	}
	// The netlist group carries the format-neutral identity.
	if !hasRow(got, "components", 2) || !hasRow(got, "nets", 3) {
		t.Errorf("StatsLines netlist counts wrong:\n%s", got)
	}
	// Structural (tier 1) counts land in format detail.
	for _, r := range []struct {
		label string
		n     int
	}{{"libraries", 1}, {"sections", 3}, {"multi-section", 1}} {
		if !hasRow(got, r.label, r.n) {
			t.Errorf("StatsLines missing %s: %d in:\n%s", r.label, r.n, got)
		}
	}
	// Netlist appears before format detail.
	if strings.Index(got, "netlist:") > strings.Index(got, "format detail:") {
		t.Errorf("netlist group should precede format detail:\n%s", got)
	}
	// Physical rows are omitted when no reader populated them (C9).
	if strings.Contains(got, "footprints:") {
		t.Errorf("StatsLines should omit footprints for a netlist-only design:\n%s", got)
	}
}

func TestStatsLinesPhysicalTier(t *testing.T) {
	d := &ir.Design{
		Name:       "P",
		Footprints: []*ir.Footprint{{Name: "R_0603"}},
		Layers:     []*ir.Layer{{Name: "F.Cu"}, {Name: "B.Cu"}},
		Stackup:    &ir.Stackup{Layers: []*ir.StackupLayer{{LayerRef: "F.Cu"}}},
		Bom:        []*ir.BomLine{{Mpn: "RES-10K"}},
	}
	got := StatsLines(d)
	for _, r := range []struct {
		label string
		n     int
	}{{"footprints", 1}, {"layers", 2}, {"stackup layers", 1}, {"bom lines", 1}} {
		if !hasRow(got, r.label, r.n) {
			t.Errorf("StatsLines missing physical-tier row %s: %d in:\n%s", r.label, r.n, got)
		}
	}
	// Zero-valued structural rows are omitted (no libraries/sections here).
	if strings.Contains(got, "libraries:") || strings.Contains(got, "sections:") {
		t.Errorf("StatsLines should omit zero structural rows:\n%s", got)
	}
}

func TestNetLines(t *testing.T) {
	d := &ir.Design{Nets: []*ir.Net{
		{Name: "GND", Connections: []*ir.Connection{
			{ComponentRef: "R1", PinRef: "2"}, {ComponentRef: "R2", PinRef: "2"},
		}},
		{Name: "SIG", Connections: []*ir.Connection{{ComponentRef: "R1", PinRef: "1"}}},
	}}
	got := NetLines(d, 0)
	if !strings.Contains(got, "R1.2, R2.2") {
		t.Errorf("NetLines missing GND membership:\n%s", got)
	}
	if !strings.Contains(got, "SIG:") {
		t.Errorf("NetLines missing SIG net:\n%s", got)
	}
	if strings.Contains(got, "more net(s)") {
		t.Errorf("NetLines should not truncate with limit 0:\n%s", got)
	}

	// limit truncates and reports the remainder.
	trunc := NetLines(d, 1)
	if !strings.Contains(trunc, "... and 1 more net(s)") {
		t.Errorf("NetLines(limit=1) should report 1 more net:\n%s", trunc)
	}
}

func TestFindingsLinesEmpty(t *testing.T) {
	if got := FindingsLines(nil); got != "no findings" {
		t.Errorf("FindingsLines(nil) = %q, want %q", got, "no findings")
	}
}

func TestFindingsLines(t *testing.T) {
	fs := []check.Finding{
		{Severity: "info", Rule: "single-pin-net", Subject: "STUB", Message: "net has 1 connection(s); expected >= 2"},
		{Severity: "warning", Rule: "unconnected-component", Subject: "C1", Message: "component has no net connections"},
		{Severity: "error", Rule: "i2c-pull-up", Subject: "SCL", Message: "I2C net has no pull-up resistor"},
	}
	got := FindingsLines(fs)
	for _, want := range []string{
		"findings by rule:",
		"single-pin-net",
		"unconnected-component",
		"i2c-pull-up",
		"[info] single-pin-net: STUB (net has 1 connection(s); expected >= 2)",
		"[warning] unconnected-component: C1 (component has no net connections)",
		"[error] i2c-pull-up: SCL (I2C net has no pull-up resistor)",
		"3 finding(s) total",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("FindingsLines missing %q in:\n%s", want, got)
		}
	}
}

// TestBundledFixtureFindings sanity-checks that a bundled fixture trips the rule it is meant
// to teach. Assert the findings directly (fields, not the formatted string): rule behavior is
// what matters here, and it should not be coupled to FindingsLines' output format.
func TestBundledFixtureFindings(t *testing.T) {
	d, err := ReadFixture("demo-board.kicad_pcb")
	if err != nil {
		t.Fatalf("ReadFixture: %v", err)
	}
	fs := check.RunDesign(d)
	if len(fs) != 1 {
		t.Fatalf("check.Run = %d findings, want 1:\n%+v", len(fs), fs)
	}
	if got := fs[0]; got.Rule != "single-pin-net" || got.Severity != "info" || got.Subject != "GND" {
		t.Errorf("finding = %+v, want rule=single-pin-net severity=info subject=GND", got)
	}
}
