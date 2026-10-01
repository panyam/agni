// Package ident compares identifiers that name the same thing in two documents that spell it
// differently: a pin map against a netlist, a selected function against a vendor's pin table.
//
// That comparison is where false warnings come from (agni issue 517). We reproduced a shipped
// in-house checker against a large production board, and EVERY warning that run produced was a
// string-comparison artifact rather than a design defect. The pin was legal in all of them and the
// two strings naming it disagreed.
//
// ONE canonical form, defined once, applied to BOTH sides. That tool carried two normalizers with
// opposite conventions (one stripped leading zeros, PTA00 to PTA0; the other padded to two digits,
// PTE7 to PTE07) on the same field, and it worked only because each ran on its own side of a
// comparison. Everything here goes through Canonical, and a caller comparing raw strings brings
// that bug back.
package ident

import (
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Match is how closely two identifiers agree. The three positive answers are kept apart because a
// reviewer does something different about each, and a bool would make a checker either noisy or
// silently permissive.
type Match string

const (
	// Exact means the two strings are byte-identical. Nothing was done to reach this.
	Exact Match = "exact"
	// Normalized means equal once both sides were canonicalized. It is a real match, and Note says
	// what differed ("these differ only by a character you cannot see") so the author can fix the map.
	Normalized Match = "normalized"
	// Fuzzy means equal only after factoring out tokens one side carries and the other does not. It
	// must be REPORTED rather than passed silently, because the tool inferred the match. Vendor tables
	// are inconsistent with themselves (see tokenSubsequence), so an author cannot be asked to guess
	// which spelling a checker prefers.
	Fuzzy Match = "fuzzy"
	// None means no reading of either string makes them agree.
	None Match = "none"
)

// Result is one comparison and why it came out that way.
type Result struct {
	// Match is how closely the two agree.
	Match Match
	// Note says what had to be done to reach a Normalized or Fuzzy match, phrased for the person
	// reading the finding. Empty for Exact and None.
	Note string
	// A and B are the alternatives that actually matched, which is not always the strings passed in.
	// Either side may be a multi-valued cell, and naming the half that matched lets a reviewer confirm
	// the answer without re-deriving it.
	A, B string
}

// hasFormatRunes reports whether s holds a format character (category Cf), the invisible
// characters that survive every defence people write for them.
//
// The obvious guard does not work. That in-house tool normalized with `re.sub(r'\s+', ”, s.upper())`.
// U+200B ZERO WIDTH SPACE is a FORMAT character, not whitespace, so `\s` does not match it in
// Python, in JavaScript, or in Go, where `\s` is ASCII-only and narrower still.
//
// The whole Cf category is stripped rather than a named list. The four observed in real documents
// are U+200B, U+200C, U+200D and U+FEFF, and the bidi marks in that category are the same hazard.
// U+00A0 NBSP is NOT here because it is a space separator, so NFKC turns it into an ordinary space
// and the whitespace stage removes it.
func hasFormatRunes(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return unicode.Is(unicode.Cf, r) }) >= 0
}

func stripFormatRunes(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
}

func stripSpace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

// canonIndex rewrites an identifier's trailing index into one spelling: brackets removed and leading
// zeros dropped, so PTE07, PTE7 and TXD[1] against TXD1 all settle.
//
// Leading zeros are DROPPED rather than padded to a fixed width. A width is a guess about the largest
// index a part family will ever reach, and the two normalizers we found disagreed because one of
// them had guessed. Dropping is the unique minimal form.
//
// A trailing group that is not all digits is left alone. `DATA[1:0]` is a bus range rather than an
// index, and xschem and gEDA both write that form, so rewriting it would collapse two different
// things into one.
func canonIndex(s string) string {
	if i := strings.LastIndexByte(s, '['); i >= 0 && strings.HasSuffix(s, "]") {
		if inner := s[i+1 : len(s)-1]; inner != "" && allDigits(inner) {
			s = s[:i] + inner
		}
	}
	j := len(s)
	for j > 0 && s[j-1] >= '0' && s[j-1] <= '9' {
		j--
	}
	if j == len(s) || j == 0 {
		return s // no trailing digits, or the whole string is digits (a bare number keeps its form)
	}
	n, err := strconv.Atoi(s[j:])
	if err != nil {
		return s // longer than an int; leave it rather than guess
	}
	return s[:j] + strconv.Itoa(n)
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// Canonical is the one canonical form of an identifier, and the only spelling anything in this
// package compares.
//
// The stage order matters. NFKC first, so a
// fullwidth digit lifted out of a PDF becomes the ASCII one and a non-breaking space becomes an
// ordinary space that the whitespace stage can then remove. Format characters next, since they
// survive NFKC untouched. Upper case after that, because vendor tables mix camel-case and upper-case
// spellings of one peripheral freely. Whitespace removed rather than collapsed, because a
// multi-valued cell is split by Alternatives BEFORE this runs, so no separator depends on the
// whitespace surviving. The index last, once the digits are ASCII.
func Canonical(s string) string {
	s = norm.NFKC.String(s)
	s = stripFormatRunes(s)
	s = strings.ToUpper(s)
	s = stripSpace(s)
	return canonIndex(s)
}

// altSeparators are the ways a document writes several names in one cell. All three were observed
// in real pin maps and vendor tables, the newline inside a single spreadsheet cell included.
var altSeparators = []string{"\n", "/", ","}

// Alternatives splits a cell that may name several functions into the ones it names, always keeping
// the WHOLE cell as the first alternative.
//
// Keeping the whole string makes this safe on a name that legitimately contains a separator.
// `R/W` is an ordinary pin name (read/write), and a splitter returning only the halves would compare
// `R` against a table and lose the name that was actually written. The caller reports WHICH reading
// matched.
//
// Both sides of a comparison are split by this same function, which fixes the largest class of
// false warning we measured. That tool looked for a separator only in the AVAILABLE function name,
// so a map cell reading `GPIO[34] / WKPU[8]` could not match a table listing `GPIO[34]` and
// `WKPU[8]` on separate rows, though both halves are legal on that pin.
func Alternatives(s string) []string {
	out := []string{strings.TrimSpace(s)}
	parts := []string{s}
	for _, sep := range altSeparators {
		var next []string
		for _, p := range parts {
			next = append(next, strings.Split(p, sep)...)
		}
		parts = next
	}
	seen := map[string]bool{out[0]: true}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// Compare reports whether two identifiers name the same thing, and what had to be done to say so.
//
// It tries the readings in order of how much they assume: byte equality, then canonical equality,
// then a token-subsequence match. The first that succeeds wins, so a pair that agrees exactly is
// never reported as having needed normalizing.
//
// Every alternative on the left is tried against every alternative on the right, and the result
// names the pair that matched.
func Compare(a, b string) Result {
	if a == b {
		return Result{Match: Exact, A: a, B: b}
	}
	altsA, altsB := readingsOf(a), readingsOf(b)
	for _, x := range altsA {
		for _, y := range altsB {
			if x.raw == y.raw {
				return Result{Match: Exact, A: x.raw, B: y.raw, Note: alternativeNote(a, b, x.raw, y.raw)}
			}
		}
	}
	for _, x := range altsA {
		for _, y := range altsB {
			if x.canonical == y.canonical {
				return Result{Match: Normalized, A: x.raw, B: y.raw,
					Note: joinNotes(differences(x.raw, y.raw), alternativeNote(a, b, x.raw, y.raw))}
			}
		}
	}
	// Tokens are split lazily because only this pass needs them. Splitting eagerly cost four
	// allocations on every normalized match, the ordinary case, since most map rows name one pin.
	tokA, tokB := tokensOf(altsA), tokensOf(altsB)
	for i, x := range altsA {
		for j, y := range altsB {
			if skipped, ok := tokenSubsequence(tokA[i], tokB[j]); ok {
				return Result{Match: Fuzzy, A: x.raw, B: y.raw,
					Note: joinNotes("one side carries "+strings.Join(skipped, ", ")+" and the other does not",
						differences(x.raw, y.raw), alternativeNote(a, b, x.raw, y.raw))}
			}
		}
	}
	return Result{Match: None, A: a, B: b}
}

// reading is one alternative of a cell with the derived forms the comparison loops need, computed
// once rather than per pair.
//
// Compare's loops are n x m for n + m distinct inputs, so on a cell naming three functions a side
// this is eight canonicalizations instead of sixty-four (#660). It costs one slice per side, so a
// single-valued comparison, which is most map rows, went from 20 allocations to 22 while the
// three-a-side cell went from 168 to 50. Trust the allocation counts over ns/op, which swings by a
// factor of two between runs on one machine. The two extra allocations buy one match ladder instead
// of a second short-circuit path, which would be the two-normalizers defect again.
//
// A CALLER COMPARING MANY AGAINST MANY should not reach for Compare in a nested loop at all. Two
// hundred map rows against sixteen hundred nets is 320,000 calls however fast each one is. Build a
// map keyed on Canonical once and look each candidate up, which is O(n + m). Compare is for deciding
// one pair and explaining the answer.
type reading struct {
	raw       string
	canonical string
}

func readingsOf(s string) []reading {
	alts := Alternatives(s)
	out := make([]reading, 0, len(alts))
	for _, a := range alts {
		out = append(out, reading{raw: a, canonical: Canonical(a)})
	}
	return out
}

// tokensOf splits each reading once, for the fuzzy pass alone.
func tokensOf(rs []reading) [][]string {
	out := make([][]string, len(rs))
	for i, r := range rs {
		out[i] = strings.Split(r.canonical, "_")
	}
	return out
}

// alternativeNote says which half of a multi-valued cell matched, and only when a cell really had
// several halves. A note claiming an alternative was chosen where none existed is noise.
func alternativeNote(a, b, x, y string) string {
	var parts []string
	if x != strings.TrimSpace(a) {
		parts = append(parts, strconv.Quote(x)+" of "+strconv.Quote(a))
	}
	if y != strings.TrimSpace(b) {
		parts = append(parts, strconv.Quote(y)+" of "+strconv.Quote(b))
	}
	if len(parts) == 0 {
		return ""
	}
	return "matched " + strings.Join(parts, " against ")
}

// differences names every normalization that actually altered one of the two strings, so the note
// says what the reader cannot see rather than only that something was done.
//
// It reports what was APPLIED rather than one decisive step, because two spellings usually differ
// in more than one way at once.
func differences(x, y string) string {
	var why []string
	if hasFormatRunes(x) || hasFormatRunes(y) {
		why = append(why, "an invisible character")
	}
	nx, ny := stripFormatRunes(norm.NFKC.String(x)), stripFormatRunes(norm.NFKC.String(y))
	if norm.NFKC.String(x) != x || norm.NFKC.String(y) != y {
		why = append(why, "a compatibility character")
	}
	if strings.ToUpper(nx) != nx || strings.ToUpper(ny) != ny {
		why = append(why, "letter case")
	}
	ux, uy := strings.ToUpper(nx), strings.ToUpper(ny)
	if stripSpace(ux) != ux || stripSpace(uy) != uy {
		why = append(why, "whitespace")
	}
	sx, sy := stripSpace(ux), stripSpace(uy)
	if canonIndex(sx) != sx || canonIndex(sy) != sy {
		why = append(why, "a zero-padded or bracketed index")
	}
	if len(why) == 0 {
		return ""
	}
	return "differ by " + strings.Join(why, " and ")
}

func joinNotes(parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, "; ")
}

// tokenSubsequence reports whether the shorter identifier's underscore-separated tokens appear in
// order within the longer's, naming the tokens the longer one carries extra.
//
// It answers a vendor table that is inconsistent with itself. One IOMUX sheet listed
// GMAC0_MII_RMII_RGMII_TXD[0] and GMAC0_MII_RGMII_TXD2 on the same pin, two spellings of one signal
// family differing by an inserted interface-mode token.
//
// It is STRUCTURAL and names no vendor vocabulary. Hard-coding MII, RMII and RGMII as optional would
// put one vendor's tokens in a shared library and miss the next vendor's equivalent.
//
// Both ends must match. Without that anchor TXD1 is a subsequence of GMAC0_MII_RGMII_TXD1 and every
// signal on the pin would match every other. A single token is never fuzzy-matched, since there is
// nothing to anchor.
func tokenSubsequence(x, y []string) ([]string, bool) {
	a, b := x, y
	if len(a) > len(b) {
		a, b = b, a
	}
	if len(a) < 2 || len(a) == len(b) {
		return nil, false
	}
	if a[0] != b[0] || a[len(a)-1] != b[len(b)-1] {
		return nil, false
	}
	var skipped []string
	i := 0
	for _, t := range b {
		if i < len(a) && a[i] == t {
			i++
			continue
		}
		skipped = append(skipped, t)
	}
	if i != len(a) {
		return nil, false
	}
	return skipped, true
}
