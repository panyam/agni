package classify

import "testing"

// TestClockClassesOf: every clock subtype carries the clock FAMILY tag (so a family-level rule reads
// HasClass(clock)), and the bare family carries only itself. The family is clock, not crystal — an
// oscillator must NOT carry a crystal tag (WS10-015).
func TestClockClassesOf(t *testing.T) {
	cases := []struct {
		in   ComponentClass
		want []string
	}{
		{ClassOscillator, []string{"oscillator", "clock"}},
		{ClassCrystal, []string{"crystal", "clock"}},
		{ClassCeramicResonator, []string{"ceramic_resonator", "clock"}},
		{ClassClock, []string{"clock"}},
	}
	for _, tc := range cases {
		got := ClassesOf(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("ClassesOf(%s) = %v, want %v", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("ClassesOf(%s) = %v, want %v", tc.in, got, tc.want)
				break
			}
		}
	}
	// An oscillator must never read as a crystal (the family-is-clock invariant).
	for _, tag := range ClassesOf(ClassOscillator) {
		if tag == string(ClassCrystal) {
			t.Error("ClassesOf(oscillator) must not carry the crystal tag")
		}
	}
}

// TestNormalizeDeviceClass: a free-form vendor device_class string resolves to the canonical class it
// names (alias table for synonyms, canonical-name set for exact names, identity for an unknown-but-
// meaningful value), so the datasheet path reaches the same class the keyword path would (WS10-015).
func TestNormalizeDeviceClass(t *testing.T) {
	cases := []struct {
		in   string
		want ComponentClass
	}{
		{"ceramic resonator", ClassCeramicResonator}, // spaced synonym
		{"Ceramic-Resonator", ClassCeramicResonator}, // punctuation + case
		{"SPXO", ClassOscillator},                    // packaged oscillator abbreviation
		{"XO", ClassOscillator},
		{"TCXO", ClassOscillator},
		{"crystal", ClassCrystal}, // canonical name (exact)
		{"oscillator", ClassOscillator},
		{"efuse", ComponentClass("efuse")}, // unknown-but-meaningful -> identity pass-through
		{"", ClassUnknown},
	}
	for _, tc := range cases {
		if got := NormalizeDeviceClass(tc.in); got != tc.want {
			t.Errorf("NormalizeDeviceClass(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}
