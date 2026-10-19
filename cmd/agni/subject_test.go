package main

import (
	"strings"
	"testing"
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
