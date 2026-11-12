package latin1

import "testing"

func TestEachByteIsItsOwnCodePoint(t *testing.T) {
	if got := Decode([]byte{'R', '1', 0xB5, 'F'}); got != "R1µF" {
		t.Errorf("Decode %q, want R1µF", got)
	}
}
