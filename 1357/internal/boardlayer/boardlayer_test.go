package boardlayer

import "testing"

func TestCopperNamesRunTopToBottom(t *testing.T) {
	got := []string{CopperName(0, 4), CopperName(1, 4), CopperName(2, 4), CopperName(3, 4), CopperName(0, 1)}
	want := []string{"F.Cu", "In1.Cu", "In2.Cu", "B.Cu", "F.Cu"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("CopperName case %d: %q, want %q", i, got[i], want[i])
		}
	}
}
