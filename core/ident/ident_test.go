package ident

import (
	"regexp"
	"strings"
	"testing"
)

// The four cases below were observed in a real run of a shipped in-house checker against a large
// production board. Every warning that run produced was a string-comparison artifact and none was a
// design defect, so these are the failure modes rather than hypotheticals.
func TestObservedFalseWarnings(t *testing.T) {
	for _, c := range []struct {
		name, a, b string
		want       Match
		note       string // a substring the note must carry, so a match cannot pass silently
	}{
		{
			name: "zero padding",
			a:    "PTE7", b: "PTE07",
			want: Normalized, note: "index",
		},
		{
			name: "an invisible character",
			a:    "ADC0_SE12\u200b", b: "ADC0_SE12", // a trailing ZERO WIDTH SPACE, written as an escape so an editor cannot silently drop it
			want: Normalized, note: "invisible",
		},
		{
			name: "a multi-valued cell against separate rows",
			a:    "GPIO[34] / WKPU[8]", b: "WKPU[8]",
			want: Exact, note: "WKPU[8]",
		},
		{
			name: "a vendor table inconsistent with itself",
			a:    "GMAC0_MII_RGMII_TXD1", b: "GMAC0_MII_RMII_RGMII_TXD[1]",
			want: Fuzzy, note: "RMII",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := Compare(c.a, c.b)
			if got.Match != c.want {
				t.Errorf("Compare(%q, %q) = %s, want %s (note %q)", c.a, c.b, got.Match, c.want, got.Note)
			}
			if !strings.Contains(got.Note, c.note) {
				t.Errorf("note = %q, want it to mention %q: a match the reader cannot check is the thing being fixed", got.Note, c.note)
			}
		})
	}
}

// The defence everyone writes for invisible characters, and why it does not work. That tool
// normalized with `\s+` removal and `.upper()`, which reads like it handles this. U+200B is a FORMAT
// character rather than whitespace, so `\s` walks straight past it, and Go's `\s` is ASCII-only so
// it is narrower still.
//
// This is a test about the standard library rather than about our code, which is the point: it fails
// the moment someone simplifies stripFormatRunes back to a whitespace strip.
func TestWhitespaceStrippingDoesNotCatchFormatCharacters(t *testing.T) {
	const zwsp = "\u200b"
	if regexp.MustCompile(`\s`).MatchString(zwsp) {
		t.Fatal("Go's \\s now matches U+200B; the reasoning in stripFormatRunes needs rewriting")
	}
	if got := strings.TrimSpace("SDA" + zwsp); got == "SDA" {
		t.Fatal("TrimSpace now removes U+200B; the reasoning in stripFormatRunes needs rewriting")
	}
	if Canonical("SDA"+zwsp) != "SDA" {
		t.Errorf("Canonical did not remove U+200B: %q", Canonical("SDA"+zwsp))
	}
}

// One canonical output, and it drops leading zeros rather than padding to a width. That tool carried
// two normalizers with opposite conventions on this exact field, which worked only because each ran
// on its own side of a comparison.
func TestCanonicalIsOneFormAndGuessesNoWidth(t *testing.T) {
	for _, c := range [][2]string{
		{"PTE07", "PTE7"},
		{"PTE7", "PTE7"},
		{"PTA00", "PTA0"},
		{"TXD[1]", "TXD1"},
		{"TXD[01]", "TXD1"},
		{"txd1", "TXD1"},
		{"PTC 11", "PTC11"},
		{"PTC\u00a011", "PTC11"}, // a NON-BREAKING space: NFKC makes it ordinary, then it is removed
	} {
		if got := Canonical(c[0]); got != c[1] {
			t.Errorf("Canonical(%q) = %q, want %q", c[0], got, c[1])
		}
	}
	if Canonical("PTE7") != Canonical("PTE07") {
		t.Error("the two spellings of one pin do not share a canonical form")
	}
}

// A bus RANGE is not an index and must not be rewritten into one. xschem and gEDA both write
// DATA[1:0], and collapsing it would make a two-bit bus compare equal to a scalar.
func TestCanonicalLeavesABusRangeAlone(t *testing.T) {
	if got := Canonical("DATA[1:0]"); got != "DATA[1:0]" {
		t.Errorf("Canonical(DATA[1:0]) = %q, want it left alone", got)
	}
	if Compare("DATA[1:0]", "DATA1").Match != None {
		t.Error("a bus range matched a scalar index")
	}
}

// A pin name may legitimately contain a separator. R/W is read/write, and a splitter that returned
// only the halves would lose the name that was actually written and match R against a table on its
// own. Keeping the whole cell as the first alternative is what makes both readings available.
func TestAlternativesKeepsAWholeNameThatContainsASeparator(t *testing.T) {
	alts := Alternatives("R/W")
	if len(alts) == 0 || alts[0] != "R/W" {
		t.Fatalf("Alternatives(R/W) = %v, want the whole name first", alts)
	}
	if Compare("R/W", "R/W").Match != Exact {
		t.Error("R/W does not match itself exactly")
	}
	// The halves stay available, and a match on one is REPORTED as such rather than passing as
	// though the whole name matched.
	got := Compare("R/W", "W")
	if got.Match != Exact || !strings.Contains(got.Note, "of \"R/W\"") {
		t.Errorf("Compare(R/W, W) = %s note %q, want a match naming which half", got.Match, got.Note)
	}
}

// Both sides are split by the same rule. That tool looked for a separator in the AVAILABLE name and
// never in the SELECTED one, which was the largest single class of false warning in the run we
// measured, so the asymmetric case is asserted in both directions here.
func TestBothSidesAreSplit(t *testing.T) {
	if got := Compare("GPIO[34] / WKPU[8]", "WKPU[8]"); got.Match == None {
		t.Error("a multi-valued cell on the LEFT did not match a single name on the right")
	}
	if got := Compare("WKPU[8]", "GPIO[34] / WKPU[8]"); got.Match == None {
		t.Error("a multi-valued cell on the RIGHT did not match a single name on the left")
	}
}

// Fuzzy matching allows an inserted MIDDLE token and nothing else, because the first token says
// which peripheral instance this is and the last says which signal and bit. Both cases below are
// genuine subsequences, so the anchor is the only thing rejecting them, which is what makes this a
// test of the anchor rather than of the subsequence walk.
//
// Written that way after a red-check: the obvious cases (a bare suffix, a differing last token)
// are rejected by the single-token rule and by the subsequence walk before the anchor is ever
// consulted, so a version with the anchor deleted passed them both.
func TestFuzzyMatchIsAnchoredAtBothEnds(t *testing.T) {
	if got := Compare("MII_TXD1", "GMAC0_MII_TXD1"); got.Match != None {
		t.Errorf("Compare(MII_TXD1, GMAC0_MII_TXD1) = %s, want none: the leading token names which "+
			"peripheral instance, so dropping it is not a spelling difference", got.Match)
	}
	if got := Compare("GMAC0_MII", "GMAC0_MII_TXD1"); got.Match != None {
		t.Errorf("Compare(GMAC0_MII, GMAC0_MII_TXD1) = %s, want none: the trailing token names the "+
			"signal, so a prefix of it is a different thing", got.Match)
	}
}

// A single token is never fuzzy-matched: there is nothing to anchor, so every short name would be a
// subsequence of every longer one that ends the same way.
func TestFuzzyMatchNeedsMoreThanOneToken(t *testing.T) {
	if got := Compare("TXD1", "GMAC0_MII_RGMII_TXD1"); got.Match != None {
		t.Errorf("Compare(TXD1, GMAC0_MII_RGMII_TXD1) = %s, want none: a bare suffix is not a match", got.Match)
	}
}

// The subsequence must hold in order. A token present but out of place is a different signal.
func TestFuzzyMatchRequiresTheTokensInOrder(t *testing.T) {
	if got := Compare("GMAC0_TXD1", "GMAC0_MII_RGMII_RXD1"); got.Match != None {
		t.Errorf("last tokens differ (TXD1 against RXD1) yet matched %s", got.Match)
	}
}

// An exact agreement is never reported as having needed work, so a reviewer reading a note knows
// something really was inferred.
func TestExactMatchCarriesNoNote(t *testing.T) {
	got := Compare("PTC11", "PTC11")
	if got.Match != Exact || got.Note != "" {
		t.Errorf("Compare on identical strings = %s note %q, want exact with no note", got.Match, got.Note)
	}
}

func TestUnrelatedNamesDoNotMatch(t *testing.T) {
	for _, c := range [][2]string{
		{"PTC11", "PTC12"},
		{"SDA", "SCL"},
		{"GMAC0_MII_RGMII_TXD1", "GMAC0_MII_RGMII_TXD2"},
	} {
		if got := Compare(c[0], c[1]); got.Match != None {
			t.Errorf("Compare(%q, %q) = %s note %q, want none", c[0], c[1], got.Match, got.Note)
		}
	}
}
