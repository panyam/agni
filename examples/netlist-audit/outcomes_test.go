package main

import (
	"strings"
	"testing"

	"github.com/panyam/agni/examples/common"
)

// TestEveryTableAnswersOnTheFixture: the fixture exists so each table has something in it, which is
// what makes a table that quietly stops matching show up as a failure rather than as an empty section.
func TestEveryTableAnswersOnTheFixture(t *testing.T) {
	d, err := common.Load("../common/designs/netlist-audit.tel")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	sections, err := answerSet(d, auditYAML)
	if err != nil {
		t.Fatalf("answerSet: %v", err)
	}
	if len(sections) != 9 {
		t.Fatalf("sections = %d, want the nine tables in audit.yaml", len(sections))
	}
	for _, s := range sections {
		if s.err != nil {
			t.Errorf("%s: %v", s.name, s.err)
		} else if len(s.rows) == 0 {
			t.Errorf("%s: no rows on the fixture built to give it some", s.name)
		}
	}
}

// TestTheFixtureAnswersAreTheDesignedOnes pins the cases the fixture was built around, so a change in
// a relation that moves one of them is seen here rather than in someone's review.
func TestTheFixtureAnswersAreTheDesignedOnes(t *testing.T) {
	d, err := common.Load("../common/designs/netlist-audit.tel")
	if err != nil {
		t.Fatal(err)
	}
	sections, err := answerSet(d, auditYAML)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]section{}
	for _, s := range sections {
		byName[s.name] = s
	}
	col0 := func(name string) string {
		var out []string
		for _, r := range byName[name].rows {
			out = append(out, strings.Join(r, "/"))
		}
		return strings.Join(out, ",")
	}
	for name, want := range map[string]string{
		"Net count":                      "5",
		"Ground test points":             "1",
		"Nets with no test point":        "I2C_SCL,MCU_NRST",
		"Passives probed on both nets":   "SYN-CAP-100N/C1,SYN-CAP-100N/C2,SYN-RES-4K7/R1,SYN-TVS-5V/D1",
		"Passives probed on one net":     "SYN-RES-10K/R3/VCC_3V3/MCU_NRST,SYN-RES-4K7/R2/VCC_3V3/I2C_SCL",
		"MPNs never probed on both nets": "SYN-RES-10K/R3",
	} {
		if got := col0(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}
