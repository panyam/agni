package main

import (
	"sort"
	"strings"
	"testing"
)

// findingLines is a check run's findings, one per line, sorted, without the summary.
func findingLines(out string) []string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); strings.HasPrefix(l, "[") {
			lines = append(lines, l)
		}
	}
	sort.Strings(lines)
	return lines
}

// TestODBChecksAsItsKiCadSource runs the whole catalog over the gateway board twice, from its
// .kicad_pcb and from the ODB++ job KiCad exported from it, each copied out of its project so no
// config differs. The board-tier rules (copper clearance, track width) read the copper the ODB++
// reader builds, so every finding has to match, those included (agni issue 1250).
func TestODBChecksAsItsKiCadSource(t *testing.T) {
	freshWorkspace(t)
	kicad := isolatedDesign(t, "../../examples/tutorial-project/designs/gateway/gateway.kicad_pcb")
	want := findingLines(runCLI(t, rootCmd(), "check", kicad))
	got := findingLines(runCLI(t, rootCmd(), "check", "../../examples/common/designs/odb/gateway-layout"))
	board := 0
	for _, l := range want {
		if strings.Contains(l, "copper-clearance") || strings.Contains(l, "track-width") {
			board++
		}
	}
	if board == 0 {
		t.Fatalf("the KiCad run has no board-tier finding, so it cannot show the ODB++ copper is read:\n%s", strings.Join(want, "\n"))
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("findings differ\nkicad:\n%s\nodb++:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
}

// TestDiffSchematicAgainstODBLayout compares the gateway's netlist with the layout of each revision
// as ODB++. Rev A's layout is its schematic, and rev B's adds the two I2C pull-ups and renames the
// crystal nets, which is what checking a layout against its schematic has to show.
func TestDiffSchematicAgainstODBLayout(t *testing.T) {
	freshWorkspace(t)
	edn := "../../examples/common/designs/odb/gateway.edn"
	same := runCLI(t, rootCmd(), "diff", edn, "../../examples/common/designs/odb/gateway-layout")
	if !strings.Contains(same, "Components: +0  -0  ~0") || !strings.Contains(same, "new 0  deleted 0  renamed 0  hard 0  soft 0") {
		t.Errorf("rev A's layout should match its schematic:\n%s", same)
	}
	revB := runCLI(t, rootCmd(), "diff", edn, "../../examples/common/designs/odb/gateway-rev-b-layout")
	for _, want := range []string{
		"Components: +2  -0  ~0",
		"[hard]    I2C_SCL: +[R4.2] -[]",
		"[hard]    PMIC_CORE_3V3: +[R4.1 R5.1] -[]",
		"[renamed] XTAL_IN -> CLK_IN",
	} {
		if !strings.Contains(revB, want) {
			t.Errorf("rev B's diff lacks %q:\n%s", want, revB)
		}
	}
}
