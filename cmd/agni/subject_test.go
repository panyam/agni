package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

func TestParseSubjects(t *testing.T) {
	got, err := parseSubjects([]string{"net:/CSI/I2C_SCL", "component:U7", "pin:U7.3", "pin:J1.A.1"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"net /CSI/I2C_SCL ", "component U7 ", "pin U7 3", "pin J1.A 1"}
	if len(got) != len(want) {
		t.Fatalf("parsed %d subjects, want %d", len(got), len(want))
	}
	for i, s := range got {
		if g := s.GetKind() + " " + s.GetRef() + " " + s.GetPin(); g != want[i] {
			t.Errorf("subject %d = %q, want %q", i, g, want[i])
		}
	}
	for _, bad := range []string{"U7", "net:", "pin:U7", "pin:U7.", "bus:DATA[7:0]"} {
		if _, err := parseSubjects([]string{bad}); err == nil || !strings.Contains(err.Error(), "--subject") {
			t.Errorf("parseSubjects(%q) = %v, want a --subject error", bad, err)
		}
	}
}

func TestNoteDesignWideNamesTheRulesLeftOut(t *testing.T) {
	var b bytes.Buffer
	noteDesignWide(&b, []*webapi.SkippedRule{
		{Name: "symbol-unresolved", Reason: check.DesignWideReason, DesignWide: true},
		{Name: "copper-clearance", Reason: check.NoBoardReason},
		{Name: "bus-not-modeled", Reason: check.DesignWideReason, DesignWide: true},
	})
	if got := b.String(); !strings.Contains(got, "2 rule(s)") || !strings.Contains(got, "bus-not-modeled, symbol-unresolved") || strings.Contains(got, "copper-clearance") {
		t.Errorf("note = %q, want the two design-wide rules alone", got)
	}
	b.Reset()
	noteDesignWide(&b, []*webapi.SkippedRule{{Name: "copper-clearance", Reason: check.NoBoardReason}})
	if b.Len() != 0 {
		t.Errorf("a run with no design-wide rule left out noted %q", b.String())
	}
}
